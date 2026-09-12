package storage

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestFailedLoginStatisticsAggregateWithoutIdentifiers(t *testing.T) {
	i, _ := sessionFixture(t)
	defer i.Close()
	now := time.Date(2026, 9, 12, 12, 34, 56, 0, time.UTC)
	s := newFailedLoginStatistics(i.DB(), 86400)
	s.now = func() time.Time { return now }
	s.record(LoginInvalidCredentials)
	s.record(LoginInvalidCredentials)
	s.record(LoginThrottled)
	s.record("unknown")
	s.closeAndFlush()

	values, err := ListFailedLoginStatistics(context.Background(), i.DB(), now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || values[0].Minute != now.Truncate(time.Minute) || values[0].Reason != LoginInvalidCredentials || values[0].Count != 2 || values[1].Reason != LoginThrottled || values[1].Count != 1 {
		t.Fatalf("statistics=%+v", values)
	}
	var columns int
	if err := i.DB().QueryRow(`SELECT count(*) FROM pragma_table_info('failed_login_statistics') WHERE name IN ('username','client','ip','headers','body')`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if columns != 0 {
		t.Fatalf("identifier-bearing columns=%d", columns)
	}
}

func TestFailedLoginStatisticsRetentionAndUnlimited(t *testing.T) {
	for _, test := range []struct {
		name      string
		retention int64
		wantOld   int
	}{{"one-day", 86400, 0}, {"unlimited", 0, 1}} {
		t.Run(test.name, func(t *testing.T) {
			i, _ := sessionFixture(t)
			defer i.Close()
			now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
			old := now.Add(-48 * time.Hour).Unix()
			if _, err := i.DB().Exec("INSERT INTO failed_login_statistics(minute_unix,reason,count) VALUES(?,?,1)", old, LoginMalformed); err != nil {
				t.Fatal(err)
			}
			s := newFailedLoginStatistics(i.DB(), test.retention)
			s.now = func() time.Time { return now }
			s.record(LoginUnavailable)
			s.closeAndFlush()
			var oldRows int
			if err := i.DB().QueryRow("SELECT count(*) FROM failed_login_statistics WHERE minute_unix=?", old).Scan(&oldRows); err != nil {
				t.Fatal(err)
			}
			if oldRows != test.wantOld {
				t.Fatalf("old rows=%d, want %d", oldRows, test.wantOld)
			}
		})
	}
}

func TestFailedLoginStatisticsPrunesWithoutANewFailure(t *testing.T) {
	i, _ := sessionFixture(t)
	defer i.Close()
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	old := now.Add(-48 * time.Hour).Unix()
	if _, err := i.DB().Exec("INSERT INTO failed_login_statistics VALUES(?,?,1)", old, LoginMalformed); err != nil {
		t.Fatal(err)
	}
	s := newFailedLoginStatistics(i.DB(), 86400)
	s.now = func() time.Time { return now }
	if err := s.flushCurrent(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.closeAndFlush()
	var rows int
	if err := i.DB().QueryRow("SELECT count(*) FROM failed_login_statistics").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("expired rows=%d", rows)
	}
}

func TestFailedLoginStatisticsSaturates(t *testing.T) {
	i, _ := sessionFixture(t)
	defer i.Close()
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	if _, err := i.DB().Exec("INSERT INTO failed_login_statistics VALUES(?,?,9223372036854775807)", now.Unix(), LoginMalformed); err != nil {
		t.Fatal(err)
	}
	s := newFailedLoginStatistics(i.DB(), 0)
	s.now = func() time.Time { return now }
	s.record(LoginMalformed)
	s.closeAndFlush()
	var count int64
	if err := i.DB().QueryRow("SELECT count FROM failed_login_statistics").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != int64(^uint64(0)>>1) {
		t.Fatalf("count=%d", count)
	}
}

func TestFailedLoginStatisticsRecordsLateShutdownAttempt(t *testing.T) {
	i, _ := sessionFixture(t)
	defer i.Close()
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	s := newFailedLoginStatistics(i.DB(), 86400)
	s.now = func() time.Time { return now }
	s.closeAndFlush()
	s.record(LoginThrottled)
	var count int64
	if err := i.DB().QueryRow("SELECT count FROM failed_login_statistics WHERE minute_unix=? AND reason=?", now.Unix(), LoginThrottled).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count=%d", count)
	}
}

func TestFailedLoginStatisticsRequeuesFailedMinuteTransition(t *testing.T) {
	i, _ := sessionFixture(t)
	defer i.Close()
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	s := newFailedLoginStatistics(i.DB(), 86400)
	s.now = func() time.Time { return now }
	realPersist := s.persist
	failures := 1
	s.persist = func(ctx context.Context, minute int64, counts map[string]int64, current int64) error {
		if failures > 0 {
			failures--
			return context.DeadlineExceeded
		}
		return realPersist(ctx, minute, counts, current)
	}
	s.record(LoginInvalidCredentials)
	now = now.Add(time.Minute)
	s.record(LoginThrottled) // transition flush fails and must requeue the old count
	if err := s.flushCurrent(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.closeAndFlush()
	var total int64
	if err := i.DB().QueryRow("SELECT coalesce(sum(count),0) FROM failed_login_statistics").Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("total=%d, want 2", total)
	}
}

func TestFailedLoginStatisticsPrunesInclusiveRetentionBoundary(t *testing.T) {
	i, _ := sessionFixture(t)
	defer i.Close()
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	boundary := now.Add(-24 * time.Hour).Unix()
	if _, err := i.DB().Exec("INSERT INTO failed_login_statistics VALUES(?,?,1)", boundary, LoginMalformed); err != nil {
		t.Fatal(err)
	}
	s := newFailedLoginStatistics(i.DB(), 86400)
	s.now = func() time.Time { return now }
	if err := s.flushCurrent(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.closeAndFlush()
	var rows int
	if err := i.DB().QueryRow("SELECT count(*) FROM failed_login_statistics").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("boundary rows=%d", rows)
	}
}

func TestFailedLoginStatisticsCloseWaitsForFailedTransitionRequeue(t *testing.T) {
	i, _ := sessionFixture(t)
	defer i.Close()
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	s := newFailedLoginStatistics(i.DB(), 86400)
	s.now = func() time.Time { return now }
	realPersist := s.persist
	entered := make(chan struct{})
	release := make(chan struct{})
	var first sync.Once
	s.persist = func(ctx context.Context, minute int64, counts map[string]int64, current int64) error {
		failed := false
		first.Do(func() {
			failed = true
			close(entered)
			<-release
		})
		if failed {
			return context.DeadlineExceeded
		}
		return realPersist(ctx, minute, counts, current)
	}
	s.record(LoginInvalidCredentials)
	now = now.Add(time.Minute)
	recorded := make(chan struct{})
	go func() {
		s.record(LoginThrottled)
		close(recorded)
	}()
	<-entered
	closed := make(chan struct{})
	go func() {
		s.closeAndFlush()
		close(closed)
	}()
	select {
	case <-closed:
		t.Fatal("close returned before the active record could requeue")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	<-recorded
	<-closed
	var total int64
	if err := i.DB().QueryRow("SELECT coalesce(sum(count),0) FROM failed_login_statistics").Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("total=%d, want 2", total)
	}
}
