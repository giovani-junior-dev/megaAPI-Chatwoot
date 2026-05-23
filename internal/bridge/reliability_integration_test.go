//go:build integration

package bridge

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// fastBackoffs swaps the package-level retryBackoff to near-zero so retry-path
// integration tests do not sit in jitter sleeps for ~36 seconds total.
func fastBackoffs(t *testing.T) {
	t.Helper()
	prev := retryBackoff
	retryBackoff = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { retryBackoff = prev })
}

func TestRunJob_RetriableUpstreamRecoversAfterRetries(t *testing.T) {
	fastBackoffs(t)
	db := setupDB(t)
	key := RandomBytes(32)
	tn := makeAuthedTenant(t, db, key, "demo-retry-ok", "b", "h")
	s := &Server{Key: key, DB: db, Cfg: Config{BufferLimit: 4}, Log: zerolog.Nop()}

	id, _, err := db.InsertMessage(context.Background(), Message{
		TenantID: tn.ID, Direction: directionIn, ExternalID: "retry-ok-1",
		Payload: []byte(`{}`),
	})
	require.NoError(t, err)

	var calls atomic.Int32
	fn := func(_ context.Context, _ Job) error {
		if calls.Add(1) < 3 {
			return retriable(errors.New("upstream 503"))
		}
		return nil
	}
	s.runJob(context.Background(),
		Job{TenantID: tn.ID, MessageID: id, Direction: directionIn, Payload: []byte(`{}`)},
		fn)

	var status string
	var attempts int
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT status, attempts FROM messages WHERE id = $1`, id).
		Scan(&status, &attempts))
	require.Equal(t, "done", status)
	require.Equal(t, int32(3), calls.Load(), "retry path must call fn until success")
	require.Equal(t, 3, attempts, "IncrementAttempts must run on each retry")
}

func TestRunJob_RetriableExhaustsAndMarksFailed(t *testing.T) {
	fastBackoffs(t)
	db := setupDB(t)
	key := RandomBytes(32)
	tn := makeAuthedTenant(t, db, key, "demo-retry-exhaust", "b", "h")
	s := &Server{Key: key, DB: db, Cfg: Config{BufferLimit: 4}, Log: zerolog.Nop()}

	id, _, err := db.InsertMessage(context.Background(), Message{
		TenantID: tn.ID, Direction: directionIn, ExternalID: "retry-x-1",
		Payload: []byte(`{}`),
	})
	require.NoError(t, err)

	var calls atomic.Int32
	fn := func(_ context.Context, _ Job) error {
		calls.Add(1)
		return retriable(errors.New("upstream 500"))
	}
	s.runJob(context.Background(),
		Job{TenantID: tn.ID, MessageID: id, Direction: directionIn, Payload: []byte(`{}`)},
		fn)

	var status, lastErr string
	var attempts int
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT status, attempts, COALESCE(last_error,'') FROM messages WHERE id = $1`, id).
		Scan(&status, &attempts, &lastErr))
	require.Equal(t, "failed", status, "retry exhaustion must mark failed")
	require.Equal(t, int32(4), calls.Load(), "should attempt initial + 3 retries")
	require.Equal(t, 4, attempts, "attempts persisted across all retries")
	require.Contains(t, lastErr, "upstream 500")
}

func TestRunJob_FatalErrorSkipsRetries(t *testing.T) {
	fastBackoffs(t)
	db := setupDB(t)
	key := RandomBytes(32)
	tn := makeAuthedTenant(t, db, key, "demo-fatal", "b", "h")
	s := &Server{Key: key, DB: db, Cfg: Config{BufferLimit: 4}, Log: zerolog.Nop()}

	id, _, err := db.InsertMessage(context.Background(), Message{
		TenantID: tn.ID, Direction: directionIn, ExternalID: "fatal-1",
		Payload: []byte(`{}`),
	})
	require.NoError(t, err)

	var calls atomic.Int32
	fn := func(_ context.Context, _ Job) error {
		calls.Add(1)
		return notRetriable(errors.New("400 bad request"))
	}
	s.runJob(context.Background(),
		Job{TenantID: tn.ID, MessageID: id, Direction: directionIn, Payload: []byte(`{}`)},
		fn)

	require.Equal(t, int32(1), calls.Load(), "fatal must short-circuit retry loop")

	var status string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT status FROM messages WHERE id = $1`, id).Scan(&status))
	require.Equal(t, "failed", status)
}

func TestRecoverPending_DropsDoneAndFailed(t *testing.T) {
	db := setupDB(t)
	key := RandomBytes(32)
	tn := makeAuthedTenant(t, db, key, "demo-recover", "b", "h")

	pendingIn, _, err := db.InsertMessage(context.Background(), Message{
		TenantID: tn.ID, Direction: directionIn, ExternalID: "p-in",
		Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	pendingOut, _, err := db.InsertMessage(context.Background(), Message{
		TenantID: tn.ID, Direction: directionOut, ExternalID: "p-out",
		Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	doneID, _, err := db.InsertMessage(context.Background(), Message{
		TenantID: tn.ID, Direction: directionIn, ExternalID: "done-1",
		Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	require.NoError(t, db.MarkStatus(context.Background(), doneID, "done", ""))
	failedID, _, err := db.InsertMessage(context.Background(), Message{
		TenantID: tn.ID, Direction: directionIn, ExternalID: "failed-1",
		Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	require.NoError(t, db.MarkStatus(context.Background(), failedID, "failed", "x"))

	s := newServerWithDB(db, key, 8)
	require.NoError(t, s.RecoverPending(context.Background()))
	require.Equal(t, 1, len(s.Inbox), "only pending inbox message should be replayed")
	require.Equal(t, 1, len(s.Outbox), "only pending outbox message should be replayed")

	gotIn := <-s.Inbox
	gotOut := <-s.Outbox
	require.Equal(t, pendingIn, gotIn.MessageID)
	require.Equal(t, pendingOut, gotOut.MessageID)
	require.Equal(t, directionIn, gotIn.Direction)
	require.Equal(t, directionOut, gotOut.Direction)
}

func TestInsertMessage_DuplicateOnConflictReturnsCreatedFalse(t *testing.T) {
	db := setupDB(t)
	key := RandomBytes(32)
	tn := makeAuthedTenant(t, db, key, "demo-dedup", "b", "h")

	first, created, err := db.InsertMessage(context.Background(), Message{
		TenantID: tn.ID, Direction: directionIn, ExternalID: "dup-key-1",
		Payload: []byte(`{"v":1}`),
	})
	require.NoError(t, err)
	require.True(t, created, "first insert must be created")

	second, created2, err := db.InsertMessage(context.Background(), Message{
		TenantID: tn.ID, Direction: directionIn, ExternalID: "dup-key-1",
		Payload: []byte(`{"v":2}`),
	})
	require.NoError(t, err)
	require.False(t, created2, "second insert with same key must report not created")
	require.NotEqual(t, first, second, "duplicate path returns Nil uuid, not the existing id")

	// Confirm original payload preserved (ON CONFLICT DO NOTHING ⇒ no overwrite).
	var payload []byte
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT payload FROM messages WHERE id = $1`, first).Scan(&payload))
	require.JSONEq(t, `{"v":1}`, string(payload))
}

func TestInsertMessage_SameExternalIDDifferentDirectionBothInserted(t *testing.T) {
	db := setupDB(t)
	key := RandomBytes(32)
	tn := makeAuthedTenant(t, db, key, "demo-dedup-dir", "b", "h")

	inID, createdIn, err := db.InsertMessage(context.Background(), Message{
		TenantID: tn.ID, Direction: directionIn, ExternalID: "cross-dir-1",
		Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	require.True(t, createdIn)

	outID, createdOut, err := db.InsertMessage(context.Background(), Message{
		TenantID: tn.ID, Direction: directionOut, ExternalID: "cross-dir-1",
		Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	require.True(t, createdOut, "same external_id under different direction must insert")
	require.NotEqual(t, inID, outID)
}

func TestSweepStale_PromotesOldPendingOnly(t *testing.T) {
	db := setupDB(t)
	key := RandomBytes(32)
	tn := makeAuthedTenant(t, db, key, "demo-sweep", "b", "h")

	oldID, _, err := db.InsertMessage(context.Background(), Message{
		TenantID: tn.ID, Direction: directionIn, ExternalID: "old-pending",
		Payload: []byte(`{}`),
	})
	require.NoError(t, err)
	_, err = db.Pool.Exec(context.Background(),
		`UPDATE messages SET created_at = NOW() - INTERVAL '2 hours' WHERE id = $1`, oldID)
	require.NoError(t, err)

	freshID, _, err := db.InsertMessage(context.Background(), Message{
		TenantID: tn.ID, Direction: directionIn, ExternalID: "fresh-pending",
		Payload: []byte(`{}`),
	})
	require.NoError(t, err)

	n, err := db.SweepStale(context.Background(), time.Hour)
	require.NoError(t, err)
	require.Equal(t, int64(1), n, "only the old pending row should be swept")

	var oldStatus, freshStatus string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT status FROM messages WHERE id = $1`, oldID).Scan(&oldStatus))
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT status FROM messages WHERE id = $1`, freshID).Scan(&freshStatus))
	require.Equal(t, "failed", oldStatus)
	require.Equal(t, "pending", freshStatus)
}

func TestFailedMessages_OrdersByCreatedAtDescending(t *testing.T) {
	db := setupDB(t)
	key := RandomBytes(32)
	tn := makeAuthedTenant(t, db, key, "demo-failed-order", "b", "h")

	var ids []string
	for i := 0; i < 3; i++ {
		id, _, err := db.InsertMessage(context.Background(), Message{
			TenantID: tn.ID, Direction: directionIn,
			ExternalID: fmt.Sprintf("fl-%d", i), Payload: []byte(`{}`),
		})
		require.NoError(t, err)
		require.NoError(t, db.MarkStatus(context.Background(), id, "failed", "boom"))
		ids = append(ids, id.String())
		time.Sleep(10 * time.Millisecond)
	}

	got, err := db.FailedMessages(context.Background(), 10)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(got), 3)
	// Newest first: ids[2] expected at index 0 within this tenant's rows.
	for i := 0; i+1 < len(got); i++ {
		require.False(t, got[i].CreatedAt.Before(got[i+1].CreatedAt),
			"FailedMessages must be ordered created_at DESC")
	}
}
