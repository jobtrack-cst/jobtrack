package monitoring_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func readYAML(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		t.Fatal(err)
	}
}

func TestPrometheus_CoversEveryComposeApplicationExactlyOnce(t *testing.T) {
	var compose struct {
		Services map[string]struct {
			Build       struct{ Dockerfile string }
			Environment map[string]string
		}
	}
	readYAML(t, "../docker-compose.yml", &compose)
	expected := map[string]bool{}
	for name, svc := range compose.Services {
		if strings.HasPrefix(svc.Build.Dockerfile, "services/") || name == "gateway" {
			if svc.Environment["HTTP_PORT"] != "8080" {
				t.Fatalf("%s HTTP port changed", name)
			}
			expected[name] = true
		}
	}
	if len(expected) != 21 {
		t.Fatalf("expected 21 applications, got %d", len(expected))
	}
	var prom struct {
		ScrapeConfigs []struct {
			JobName       string `yaml:"job_name"`
			MetricsPath   string `yaml:"metrics_path"`
			StaticConfigs []struct {
				Targets []string
				Labels  map[string]string
			} `yaml:"static_configs"`
		} `yaml:"scrape_configs"`
	}
	readYAML(t, "prometheus.yml", &prom)
	if len(prom.ScrapeConfigs) != 1 {
		t.Fatal("expected one application scrape job")
	}
	job := prom.ScrapeConfigs[0]
	if job.JobName != "jobtrack" || job.MetricsPath != "/metrics" {
		t.Fatal("unexpected scrape job/path")
	}
	for _, cfg := range job.StaticConfigs {
		name := cfg.Labels["service"]
		if !expected[name] || len(cfg.Targets) != 1 || cfg.Targets[0] != name+":8080" {
			t.Fatalf("invalid/duplicate target: %+v", cfg)
		}
		delete(expected, name)
	}
	if len(expected) != 0 {
		t.Fatalf("missing services: %v", expected)
	}
}

func TestDashboard_UsesRequestMetricsAndCorrectAggregations(t *testing.T) {
	data, err := os.ReadFile("grafana/dashboards/jobtrack.json")
	if err != nil {
		t.Fatal(err)
	}
	var dashboard struct {
		UID    string
		Panels []struct {
			Title, Type string
			Targets     []struct{ Expr string }
			Datasource  struct{ UID string }
			Options     struct{ Content string }
		}
	}
	if err := json.Unmarshal(data, &dashboard); err != nil {
		t.Fatal(err)
	}
	if dashboard.UID != "jobtrack-compose" || len(dashboard.Panels) != 6 {
		t.Fatal("unexpected dashboard")
	}
	for _, panel := range dashboard.Panels {
		if panel.Type != "timeseries" || strings.Contains(panel.Title, "BLOCKED") {
			t.Fatal("expected live charts")
		}
		if panel.Datasource.UID != "jobtrack-prometheus" || len(panel.Targets) != 1 {
			t.Fatal("invalid datasource/query")
		}
		expr := panel.Targets[0].Expr
		if !strings.Contains(expr, `job="jobtrack"`) || !strings.Contains(expr, `service=~"$service"`) {
			t.Fatal("missing target filters")
		}
	}
	for i, metric := range []string{"jobtrack_requests_total", "result=\"error\"", "jobtrack_request_duration_seconds_sum"} {
		expr := dashboard.Panels[i].Targets[0].Expr
		if !strings.Contains(expr, metric) || !strings.Contains(expr, "sum by (service)") || !strings.Contains(expr, "$__rate_interval") {
			t.Fatalf("invalid request query %s", expr)
		}
	}
	if !strings.Contains(dashboard.Panels[1].Targets[0].Expr, "or on (service)") || !strings.Contains(dashboard.Panels[1].Targets[0].Expr, "> 0") || !strings.Contains(dashboard.Panels[2].Targets[0].Expr, "> 0") {
		t.Fatal("missing idle/error handling")
	}
	var source struct{ Datasources []struct{ UID, URL string } }
	readYAML(t, "grafana/provisioning/datasources/prometheus.yml", &source)
	if len(source.Datasources) != 1 || source.Datasources[0].UID != "jobtrack-prometheus" || source.Datasources[0].URL != "http://prometheus:9090" {
		t.Fatal("invalid datasource provisioning")
	}
	var providers struct {
		Providers []struct{ Options struct{ Path string } }
	}
	readYAML(t, "grafana/provisioning/dashboards/jobtrack.yml", &providers)
	if len(providers.Providers) != 1 || providers.Providers[0].Options.Path != "/var/lib/grafana/dashboards" {
		t.Fatal("invalid dashboard provisioning")
	}
	files, err := filepath.Glob("grafana/dashboards/*.json")
	if err != nil || len(files) != 1 {
		t.Fatal("expected one provisioned dashboard")
	}
}

func TestGrafana_RejectsUnsafeBootstrapCredentialsWithoutDisclosure(t *testing.T) {
	for _, tc := range []struct{ name, user, password string }{
		{"missing", "", ""},
		{"example user", "change-me", "synthetic-valid-password"},
		{"example password", "synthetic-user", "change-me"},
		{"short password", "synthetic-user", "short-value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("sh", "start-grafana.sh")
			cmd.Env = []string{"PATH=/usr/bin:/bin", "GF_SECURITY_ADMIN_USER=" + tc.user, "GF_SECURITY_ADMIN_PASSWORD=" + tc.password}
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatal("unsafe credentials accepted")
			}
			if tc.password != "" && strings.Contains(string(out), tc.password) {
				t.Fatal("credential disclosed")
			}
			if !strings.Contains(string(out), "GRAFANA_ADMIN_") {
				t.Fatalf("missing diagnostic: %s", out)
			}
		})
	}
}
