package fswalker_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	tspb "google.golang.org/protobuf/types/known/timestamppb"

	fspb "github.com/google/fswalker/proto/fswalker"
)

func TestReadCompatProtoData(t *testing.T) {
	if os.Getenv("FSWALKER_GENERATE_COMPAT_PROTO") != "" {
		// re-generate test data
		writeCompatProtoData(t)
	}

	cases := []struct {
		protoType string
		want      proto.Message
	}{
		{
			protoType: "walk",
			want:      getTestWalk(),
		},
		{
			protoType: "policy",
			want:      getTestPolicy(),
		},
		{
			protoType: "report-config",
			want:      getTestReportConfig(),
		},
		{
			protoType: "reviews",
			want:      getTestReviews(),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("read-compat-"+tc.protoType, func(t *testing.T) {
			got := proto.Clone(tc.want)
			got.Reset()

			got = readProtoBin(t, got, "testdata/proto-1-5-3-compat-"+tc.protoType+".pb")

			if !proto.Equal(tc.want, got) {
				t.Errorf("%s marshalled binary message did not match: %s", tc.protoType, cmp.Diff(tc.want, got, cmp.Comparer(proto.Equal)))
			}

			got.Reset()
			readProtoText(t, got, "testdata/proto-1-5-3-compat-"+tc.protoType+".textpb")

			if !proto.Equal(tc.want, got) {
				t.Errorf("%s marshalled text message did not match: %s", tc.protoType, cmp.Diff(tc.want, got, cmp.Comparer(proto.Equal)))
			}
		})
	}
}

func writeCompatProtoData(t *testing.T) {
	t.Helper()

	t.Run("write-compat-walk", func(t *testing.T) {
		w := getTestWalk()

		writeProtoBin(t, w, "testdata/proto-1-5-3-compat-walk.pb")
		writeProtoText(t, w, "testdata/proto-1-5-3-compat-walk.textpb")
	})

	t.Run("write-compat-policy", func(t *testing.T) {
		p := getTestPolicy()

		writeProtoBin(t, p, "testdata/proto-1-5-3-compat-policy.pb")
		writeProtoText(t, p, "testdata/proto-1-5-3-compat-policy.textpb")
	})

	t.Run("write-compat-report-config", func(t *testing.T) {
		c := getTestReportConfig()

		writeProtoBin(t, c, "testdata/proto-1-5-3-compat-report-config.pb")
		writeProtoText(t, c, "testdata/proto-1-5-3-compat-report-config.textpb")
	})

	t.Run("write-compat-reviews", func(t *testing.T) {
		reviews := getTestReviews()

		writeProtoBin(t, reviews, "testdata/proto-1-5-3-compat-reviews.pb")
		writeProtoText(t, reviews, "testdata/proto-1-5-3-compat-reviews.textpb")
	})
}

func getTestPolicy() *fspb.Policy {
	return &fspb.Policy{
		Version: 1,
		Include: []string{
			"/",
		},
		ExcludePfx: []string{
			"/var/log/",
			"/home/",
			"/tmp/",
		},
		HashPfx: []string{
			"/etc/",
		},
		IgnoreIrregularFiles: true,
		MaxHashFileSize:      1024 * 1024,
		MaxDirectoryDepth:    1265,
		WalkCrossDevice:      true,
	}
}

func getTestWalk() *fspb.Walk {
	return &fspb.Walk{
		Id:        "3ca83f34-0a8f-4fb5-9c94-c542fb06de35",
		Version:   1,
		Hostname:  "testhost",
		StartWalk: tspb.New(time.Date(2023, 02, 21, 07, 34, 12, 433242, time.UTC)),
		StopWalk:  tspb.New(time.Date(2023, 02, 21, 07, 35, 48, 122488, time.UTC)),
		Policy:    getTestPolicy(),
		File: []*fspb.File{
			{
				Version: 1,
				Path:    "/etc/test",
				Info: &fspb.FileInfo{
					Name:  "hashSumTest",
					Size:  100,
					Mode:  640,
					IsDir: false,
				},
				Fingerprint: []*fspb.Fingerprint{
					{
						Method: fspb.Fingerprint_SHA256,
						Value:  "deadbeef",
					},
				},
			},
		},
	}
}

func getTestReviews() *fspb.Reviews {
	return &fspb.Reviews{
		Review: map[string]*fspb.Review{
			"somehost": {
				WalkId:        "035457ff-a958-410a-8619-fe3e0d567bfd",
				WalkReference: "walk-file-path",
				Fingerprint: &fspb.Fingerprint{
					Method: fspb.Fingerprint_SHA256,
					Value:  "0354fe3e0d567bfd",
				},
			},
		},
	}
}

func getTestReportConfig() *fspb.ReportConfig {
	return &fspb.ReportConfig{
		Version:    1,
		ExcludePfx: []string{"/ignore/"},
	}
}

func writeProtoBin(t *testing.T, m proto.Message, filename string) {
	t.Helper()

	b, err := proto.Marshal(m)
	if err != nil {
		t.Fatal("problems marshaling proto message:", err)
	}

	if err = os.WriteFile(filename, b, 0644); err != nil {
		t.Fatal(err)
	}
}

func writeProtoText(t *testing.T, m proto.Message, filename string) {
	t.Helper()

	s := proto.MarshalTextString(m)
	s = strings.Replace(strings.Replace(s, "<", "{", -1), ">", "}", -1)

	if err := os.WriteFile(filename, []byte(s), 0644); err != nil {
		t.Fatal(err)
	}
}

func readProtoBin(t *testing.T, m proto.Message, filename string) proto.Message {
	t.Helper()

	b, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("when reading %q: %v", filename, err)
	}

	if err := proto.Unmarshal(b, m); err != nil {
		t.Fatalf("unmarshaling from %q: %v", filename, err)
	}

	return m
}

func readProtoText(t *testing.T, m proto.Message, filename string) proto.Message {
	t.Helper()

	b, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("when reading %q: %v", filename, err)
	}

	if err := proto.UnmarshalText(string(b), m); err != nil {
		t.Fatalf("unmarshaling from %q: %v", filename, err)
	}

	return m
}
