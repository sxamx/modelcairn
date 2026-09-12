package storage

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"sync"
	"time"
)

const failedLoginFlushInterval = time.Minute

type FailedLoginStatistic struct {
	Minute time.Time
	Reason string
	Count  int64
}

// failedLoginStatistics aggregates privacy-preserving counters in memory. It
// stores no client address, username, headers, request body, or free-form text.
type failedLoginStatistics struct {
	db               *sql.DB
	retentionSeconds int64
	now              func() time.Time
	persist          func(context.Context, int64, map[string]int64, int64) error

	mu     sync.Mutex
	minute int64
	counts map[string]int64

	lifecycleMu sync.Mutex
	closed      bool
	active      sync.WaitGroup
	stop        chan struct{}
	done        chan struct{}
	close       sync.Once
}

func newFailedLoginStatistics(db *sql.DB, retentionSeconds int64) *failedLoginStatistics {
	s := &failedLoginStatistics{db: db, retentionSeconds: retentionSeconds, now: time.Now, counts: make(map[string]int64, 4), stop: make(chan struct{}), done: make(chan struct{})}
	s.persist = s.flush
	go s.run()
	return s
}

func (s *failedLoginStatistics) record(reason string) {
	if !validLoginReason(reason) {
		return
	}
	minute := s.now().UTC().Unix()
	minute -= minute % 60
	s.lifecycleMu.Lock()
	if s.closed {
		s.lifecycleMu.Unlock()
		_ = s.persist(context.Background(), minute, map[string]int64{reason: 1}, minute)
		return
	}
	s.active.Add(1)
	s.lifecycleMu.Unlock()
	defer s.active.Done()

	s.mu.Lock()
	if s.minute == 0 {
		s.minute = minute
	}
	// A backwards wall-clock adjustment must not create older buckets forever.
	if minute < s.minute {
		minute = s.minute
	}
	oldMinute := int64(0)
	var old map[string]int64
	if minute > s.minute {
		oldMinute, old = s.minute, s.counts
		s.minute, s.counts = minute, make(map[string]int64, 4)
	}
	if s.counts[reason] < math.MaxInt64 {
		s.counts[reason]++
	}
	s.mu.Unlock()
	if len(old) > 0 {
		if err := s.persist(context.Background(), oldMinute, old, minute); err != nil {
			s.requeue(old)
		}
	}
}

func (s *failedLoginStatistics) run() {
	ticker := time.NewTicker(failedLoginFlushInterval)
	defer func() { ticker.Stop(); close(s.done) }()
	for {
		select {
		case <-ticker.C:
			_ = s.flushCurrent(context.Background())
		case <-s.stop:
			_ = s.flushCurrent(context.Background())
			return
		}
	}
}

func (s *failedLoginStatistics) closeAndFlush() {
	s.close.Do(func() {
		s.lifecycleMu.Lock()
		s.closed = true
		s.lifecycleMu.Unlock()
		s.active.Wait()
		close(s.stop)
		<-s.done
		s.mu.Lock()
		minute, counts := s.minute, s.counts
		s.counts = make(map[string]int64, 4)
		s.mu.Unlock()
		if len(counts) > 0 {
			_ = s.persist(context.Background(), minute, counts, s.now().UTC().Unix())
		}
	})
}

func (s *failedLoginStatistics) flushCurrent(ctx context.Context) error {
	s.mu.Lock()
	minute, counts := s.minute, s.counts
	s.counts = make(map[string]int64, 4)
	s.mu.Unlock()
	if len(counts) == 0 && s.retentionSeconds == 0 {
		return nil
	}
	if err := s.persist(ctx, minute, counts, s.now().UTC().Unix()); err != nil {
		s.requeue(counts)
		return err
	}
	return nil
}

// requeue merges a failed batch into the single current map. This bounds memory
// during a persistence outage. Counts are preserved, while their minute may be
// coalesced into the next successfully persisted bucket.
func (s *failedLoginStatistics) requeue(counts map[string]int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for reason, count := range counts {
		if count > math.MaxInt64-s.counts[reason] {
			s.counts[reason] = math.MaxInt64
		} else {
			s.counts[reason] += count
		}
	}
}

func (s *failedLoginStatistics) flush(ctx context.Context, minute int64, counts map[string]int64, nowUnix int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for reason, count := range counts {
		if count < 1 || !validLoginReason(reason) {
			continue
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO failed_login_statistics(minute_unix,reason,count) VALUES(?,?,?)
			ON CONFLICT(minute_unix,reason) DO UPDATE SET count=CASE
			WHEN count > 9223372036854775807-excluded.count THEN 9223372036854775807
			ELSE count+excluded.count END`, minute, reason, count)
		if err != nil {
			return err
		}
	}
	if s.retentionSeconds > 0 {
		cutoff := nowUnix - s.retentionSeconds
		cutoff -= cutoff % 60
		if _, err = tx.ExecContext(ctx, "DELETE FROM failed_login_statistics WHERE minute_unix <= ?", cutoff); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func validLoginReason(reason string) bool {
	switch reason {
	case LoginInvalidCredentials, LoginThrottled, LoginMalformed, LoginUnavailable:
		return true
	default:
		return false
	}
}

func ListFailedLoginStatistics(ctx context.Context, db *sql.DB, since time.Time, limit int) ([]FailedLoginStatistic, error) {
	if limit < 1 || limit > 10000 {
		return nil, errors.New("invalid_failed_login_statistics_limit")
	}
	rows, err := db.QueryContext(ctx, `SELECT minute_unix,reason,count FROM failed_login_statistics
		WHERE minute_unix >= ? ORDER BY minute_unix,reason LIMIT ?`, since.UTC().Unix(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]FailedLoginStatistic, 0)
	for rows.Next() {
		var value FailedLoginStatistic
		var minute int64
		if err := rows.Scan(&minute, &value.Reason, &value.Count); err != nil {
			return nil, err
		}
		value.Minute = time.Unix(minute, 0).UTC()
		result = append(result, value)
	}
	return result, rows.Err()
}
