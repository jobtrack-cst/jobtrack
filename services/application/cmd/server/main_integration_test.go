//go:build integration

package main

import (
	"context"
	"io"
	"net"
	"net/http"
	urlpkg "net/url"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/testcontainers/testcontainers-go"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestRun_RealDatabaseMigrationsAndHTTPHealth(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := pgcontainer.Run(ctx, "postgres:17-alpine", pgcontainer.WithDatabase("application_startup_test"), pgcontainer.WithUsername("application_test"), pgcontainer.WithPassword("synthetic-@:/?#%-password"), pgcontainer.BasicWaitStrategies())
	if container != nil {
		t.Cleanup(func() {
			cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			if err := container.Terminate(cleanup); err != nil {
				t.Error("terminate test PostgreSQL failed")
			}
		})
	}
	if err != nil {
		t.Fatal("start test PostgreSQL failed")
	}
	// Mirror Compose without asking Testcontainers to embed the raw password in a URL.
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatal("get test host failed")
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatal("get test port failed")
	}
	address := &urlpkg.URL{Scheme: "postgres", Host: net.JoinHostPort(host, port.Port()), User: urlpkg.User("application_test"), Path: "/application_startup_test", RawQuery: "sslmode=disable"}
	url := address.String()
	t.Setenv("PGPASSWORD", "synthetic-@:/?#%-password")
	grpcListener, httpListener := listenForTest(t), listenForTest(t)
	grpcPort, _ := strconv.Atoi(grpcListener.Addr().String()[len("127.0.0.1:"):])
	httpPort, _ := strconv.Atoi(httpListener.Addr().String()[len("127.0.0.1:"):])
	grpcListener.Close()
	httpListener.Close()
	broker, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "rabbitmq:4.1-alpine", ExposedPorts: []string{"5672/tcp"}, WaitingFor: wait.ForListeningPort("5672/tcp")}, Started: true})
	if broker != nil {
		t.Cleanup(func() { _ = broker.Terminate(context.Background()) })
	}
	if err != nil {
		t.Fatal(err)
	}
	brokerHost, err := broker.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	brokerPort, err := broker.MappedPort(ctx, "5672/tcp")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config{rabbitMQURL: "amqp://guest:guest@" + net.JoinHostPort(brokerHost, brokerPort.Port()) + "/", jobPostingAddr: "job-posting:9090", resumeAddr: "resume:9090", serviceName: "application", databaseURL: url, migrationsDir: filepath.Join("..", "..", "migrations"), grpcPort: grpcPort, httpPort: httpPort}
	runCtx, stop := context.WithCancel(ctx)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- run(runCtx, cfg, logging.New("application", io.Discard)) }()
	// The timer bounds startup and the ticker retries only while the server binds.
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case err := <-done:
			t.Fatalf("startup failed: %v", err)
		case <-deadline.C:
			stop()
			awaitServer(t, done)
			t.Fatal("startup health timeout")
		case <-tick.C:
			response, e := client.Get("http://127.0.0.1:" + strconv.Itoa(httpPort) + "/healthz")
			if e != nil {
				continue
			}
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if response.StatusCode != http.StatusOK || string(body) != "ok\n" {
				continue
			}
			stop()
			if err = awaitServer(t, done); err != nil {
				t.Fatal(err)
			}
			// Startup migrations are safe on a second process start as well.
			database, err := openDatabase(ctx, url)
			if err != nil {
				t.Fatal(err)
			}
			defer database.close()
			if err = database.migrate(ctx, cfg.migrationsDir); err != nil {
				t.Fatal(err)
			}
			t.Log("actual application startup applied migrations; GET /healthz returned HTTP 200")
			return
		}
	}
}
