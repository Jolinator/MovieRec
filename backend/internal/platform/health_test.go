package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"backend/internal/common"
	"backend/internal/database"
)

type fakePinger struct {
	err   error
	calls int
}

func (p *fakePinger) Ping(context.Context) error {
	p.calls++
	return p.err
}

func TestServiceHealthHandler(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		pingErr   error
		preScan   bool
		wantPings int
		wantDB    bool
	}{
		{"scan=true pings", "?scan=true", nil, false, 1, true},
		{"scan=true reports failure", "?scan=true", errors.New("down"), false, 1, false},
		{"scan=false never pings", "?scan=false", nil, false, 0, false},
		{"default rescans when down", "", nil, false, 1, true},
		{"default skips when up", "", nil, true, 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pinger := &fakePinger{err: tt.pingErr}
			status := database.NewDBStatus(pinger)
			if tt.preScan {
				status.Scan(context.Background())
			}
			handler := ServiceHealthHandler(status, func() bool { return true })

			rec := httptest.NewRecorder()
			handler(rec, httptest.NewRequest("GET", "/health"+tt.query, nil))

			var got common.ServiceHealth
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.DatabaseAvailable != tt.wantDB || !got.Ready {
				t.Fatalf("health = %+v, want db=%v ready=true", got, tt.wantDB)
			}
			if pinger.calls != tt.wantPings {
				t.Fatalf("pings = %d, want %d", pinger.calls, tt.wantPings)
			}
		})
	}
}
