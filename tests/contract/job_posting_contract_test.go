package contract_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	jobpostingv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_posting/v1"
)

// httpBinding returns the verb, path and body of a method's HTTP rule.
func httpBinding(t *testing.T, method protoreflect.MethodDescriptor) (string, string, string) {
	t.Helper()
	require.True(t, proto.HasExtension(method.Options(), annotations.E_Http), "missing HTTP binding")
	rule := proto.GetExtension(method.Options(), annotations.E_Http).(*annotations.HttpRule)
	assert.Empty(t, rule.GetResponseBody())
	assert.Empty(t, rule.GetAdditionalBindings())
	switch pattern := rule.GetPattern().(type) {
	case *annotations.HttpRule_Get:
		return "GET", pattern.Get, rule.GetBody()
	case *annotations.HttpRule_Post:
		return "POST", pattern.Post, rule.GetBody()
	case *annotations.HttpRule_Put:
		return "PUT", pattern.Put, rule.GetBody()
	case *annotations.HttpRule_Delete:
		return "DELETE", pattern.Delete, rule.GetBody()
	default:
		t.Fatalf("unexpected HTTP pattern %T", pattern)
		return "", "", ""
	}
}

func TestJobPostingContract_PublicRPCsAndHTTPBindings(t *testing.T) {
	file := jobpostingv1.File_jobtrack_job_posting_v1_job_posting_proto
	require.Equal(t, protoreflect.FullName("jobtrack.job_posting.v1"), file.Package())
	service := file.Services().ByName("JobPostingService")
	require.NotNil(t, service)

	cases := []struct {
		method, verb, path, body string
	}{
		{"CreateJob", "POST", "/v1/jobs", "*"},
		{"UpdateJob", "PUT", "/v1/jobs/{job_id}", "*"},
		{"PublishJob", "POST", "/v1/jobs/{job_id}/publish", "*"},
		{"CloseJob", "POST", "/v1/jobs/{job_id}/close", "*"},
		{"GetJob", "GET", "/v1/jobs/{job_id}", ""},
		{"ListMyJobs", "GET", "/v1/employer/jobs", ""},
		{"DeleteJob", "DELETE", "/v1/jobs/{job_id}", ""},
	}
	require.Equal(t, len(cases), service.Methods().Len(), "unexpected public method count")
	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			method := service.Methods().ByName(protoreflect.Name(tc.method))
			require.NotNil(t, method, "missing RPC")
			assert.Equal(t, protoreflect.Name(tc.method+"Request"), method.Input().Name())
			assert.Equal(t, protoreflect.Name(tc.method+"Response"), method.Output().Name())
			assert.False(t, method.IsStreamingClient() || method.IsStreamingServer(), "expected unary RPC")
			verb, path, body := httpBinding(t, method)
			assert.Equal(t, tc.verb, verb)
			assert.Equal(t, tc.path, path)
			assert.Equal(t, tc.body, body)
			// The owner always comes from the trusted identity, never the request.
			assert.Nil(t, method.Input().Fields().ByName("employer_id"), "request must not accept employer_id")
		})
	}
}

func TestJobPostingContract_InternalServiceHasNoHTTPRoutes(t *testing.T) {
	service := jobpostingv1.File_jobtrack_job_posting_v1_job_posting_proto.Services().ByName("JobPostingInternalService")
	require.NotNil(t, service)
	require.Equal(t, 1, service.Methods().Len())
	method := service.Methods().Get(0)
	assert.Equal(t, protoreflect.Name("ListJobsClosingBetween"), method.Name())
	assert.False(t, proto.HasExtension(method.Options(), annotations.E_Http), "internal RPC must have no HTTP route")
}

// JT-116: DeleteJob takes only the job ID and returns an empty response.
func TestJobPostingContract_DeleteJobMessages(t *testing.T) {
	file := jobpostingv1.File_jobtrack_job_posting_v1_job_posting_proto
	req := file.Messages().ByName("DeleteJobRequest")
	require.NotNil(t, req)
	require.Equal(t, 1, req.Fields().Len())
	field := req.Fields().ByName("job_id")
	require.NotNil(t, field)
	assert.Equal(t, protoreflect.FieldNumber(1), field.Number())
	assert.Equal(t, protoreflect.StringKind, field.Kind())
	assert.False(t, field.IsList())

	resp := file.Messages().ByName("DeleteJobResponse")
	require.NotNil(t, resp)
	assert.Zero(t, resp.Fields().Len(), "DeleteJobResponse must be empty")
}
