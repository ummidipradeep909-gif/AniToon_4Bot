package leases

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrLeaseHeld = errors.New("polling lease is held by another instance")
	ErrInvalidLease = errors.New("invalid polling lease")
)

const (
	DefaultLeaseTTL = 90 * time.Second
	DefaultRenewInterval = 20 * time.Second
	renewTimeout = 5 * time.Second
)

type Lease struct {
	BotID int64
	InstanceID string
	FencingToken int64
}

type Store struct { db *pgxpool.Pool }

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

// Acquire uses one atomic PostgreSQL upsert. A live lease is never replaced;
// takeover increments a durable fencing token after expiry.
func (s *Store) Acquire(ctx context.Context, botID int64, instanceID string, ttl time.Duration) (Lease, bool, error) {
	if s == nil || s.db == nil { return Lease{}, false, fmt.Errorf("polling lease storage is unavailable") }
	if botID <= 0 || instanceID == "" || ttl <= 0 { return Lease{}, false, ErrInvalidLease }
	var token int64
	err := s.db.QueryRow(ctx, `
		INSERT INTO bot_polling_leases (bot_id, instance_id, fencing_token, lease_expires_at, updated_at)
		VALUES ($1, $2, 1, NOW() + ($3 * INTERVAL '1 millisecond'), NOW())
		ON CONFLICT (bot_id) DO UPDATE
		SET instance_id = EXCLUDED.instance_id,
			fencing_token = bot_polling_leases.fencing_token + 1,
			lease_expires_at = EXCLUDED.lease_expires_at,
			updated_at = NOW()
		WHERE bot_polling_leases.lease_expires_at <= NOW()
		RETURNING fencing_token`, botID, instanceID, ttl.Milliseconds()).Scan(&token)
	if errors.Is(err, pgx.ErrNoRows) { return Lease{}, false, nil }
	if err != nil { return Lease{}, false, fmt.Errorf("acquire polling lease: %w", err) }
	return Lease{BotID:botID, InstanceID:instanceID, FencingToken:token}, true, nil
}

// Renew never resurrects an expired lease. Returning false means the lease is
// no longer safely owned by this process and polling must stop.
func (s *Store) Renew(ctx context.Context, lease Lease, ttl time.Duration) (bool, error) {
	if s == nil || s.db == nil { return false, fmt.Errorf("polling lease storage is unavailable") }
	if lease.BotID <= 0 || lease.InstanceID == "" || lease.FencingToken <= 0 || ttl <= 0 { return false, ErrInvalidLease }
	tag, err := s.db.Exec(ctx, `
		UPDATE bot_polling_leases
		SET lease_expires_at = NOW() + ($4 * INTERVAL '1 millisecond'), updated_at = NOW()
		WHERE bot_id = $1 AND instance_id = $2 AND fencing_token = $3 AND lease_expires_at > NOW()`,
		lease.BotID, lease.InstanceID, lease.FencingToken, ttl.Milliseconds())
	if err != nil { return false, fmt.Errorf("renew polling lease: %w", err) }
	return tag.RowsAffected() == 1, nil
}

// Release is fenced by instance ID and fencing token. A stale process cannot
// release a lease acquired later by another instance.
func (s *Store) Release(ctx context.Context, lease Lease) (bool, error) {
	if s == nil || s.db == nil { return false, fmt.Errorf("polling lease storage is unavailable") }
	if lease.BotID <= 0 || lease.InstanceID == "" || lease.FencingToken <= 0 { return false, ErrInvalidLease }
	tag, err := s.db.Exec(ctx, `
		UPDATE bot_polling_leases SET lease_expires_at = NOW(), updated_at = NOW()
		WHERE bot_id = $1 AND instance_id = $2 AND fencing_token = $3`,
		lease.BotID, lease.InstanceID, lease.FencingToken)
	if err != nil { return false, fmt.Errorf("release polling lease: %w", err) }
	return tag.RowsAffected() == 1, nil
}

// Coordinator owns all leases held by one process and stops renewing/announces
// loss immediately on any ambiguous renewal failure. The caller must stop that
// bot's updater loop from onLost before attempting a new acquisition.
type Coordinator struct {
	store *Store
	instanceID string
	ttl time.Duration
	renewInterval time.Duration
	mu sync.Mutex
	held map[int64]Lease
}

func NewCoordinator(db *pgxpool.Pool) (*Coordinator, error) {
	return newCoordinator(db, "", DefaultLeaseTTL, DefaultRenewInterval)
}

func newCoordinator(db *pgxpool.Pool, instanceID string, ttl, renewInterval time.Duration) (*Coordinator, error) {
	if db == nil { return nil, fmt.Errorf("polling lease coordinator requires PostgreSQL") }
	if ttl < 60*time.Second || renewInterval <= 0 || renewInterval >= ttl/2 {
		return nil, fmt.Errorf("polling lease timing is invalid")
	}
	if instanceID == "" {
		var raw [16]byte
		if _, err := rand.Read(raw[:]); err != nil { return nil, fmt.Errorf("create polling instance identity") }
		instanceID = hex.EncodeToString(raw[:])
	}
	return &Coordinator{store:NewStore(db), instanceID:instanceID, ttl:ttl, renewInterval:renewInterval, held:make(map[int64]Lease)}, nil
}

func (c *Coordinator) InstanceID() string { return c.instanceID }

func (c *Coordinator) Acquire(ctx context.Context, botID int64) (Lease, error) {
	c.mu.Lock()
	_, alreadyHeld := c.held[botID]
	c.mu.Unlock()
	if alreadyHeld { return Lease{}, fmt.Errorf("bot %d already has a local polling lease", botID) }
	lease, acquired, err := c.store.Acquire(ctx, botID, c.instanceID, c.ttl)
	if err != nil { return Lease{}, err }
	if !acquired { return Lease{}, ErrLeaseHeld }
	c.mu.Lock()
	c.held[botID] = lease
	c.mu.Unlock()
	return lease, nil
}

// Release removes the local renewal record before releasing the durable row.
// Call only after its updater loop has been stopped.
func (c *Coordinator) Release(ctx context.Context, botID int64) error {
	c.mu.Lock()
	lease, ok := c.held[botID]
	delete(c.held, botID)
	c.mu.Unlock()
	if !ok { return nil }
	_, err := c.store.Release(ctx, lease)
	return err
}

func (c *Coordinator) Held(botID int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.held[botID]
	return ok
}

func (c *Coordinator) snapshot() []Lease {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]Lease, 0, len(c.held))
	for _, lease := range c.held { result = append(result, lease) }
	return result
}

func (c *Coordinator) renew(ctx context.Context, lease Lease) (bool, error) {
	return c.store.Renew(ctx, lease, c.ttl)
}

func (c *Coordinator) Run(ctx context.Context, onLost func(botID int64)) {
	ticker := time.NewTicker(c.renewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, lease := range c.snapshot() {
				if ctx.Err() != nil { return }
				renewCtx, cancel := context.WithTimeout(ctx, renewTimeout)
				ok, err := c.renew(renewCtx, lease)
				cancel()
				if err == nil && ok { continue }
				c.mu.Lock()
				current, exists := c.held[lease.BotID]
				lost := exists && current.FencingToken == lease.FencingToken && current.InstanceID == lease.InstanceID
				if lost { delete(c.held, lease.BotID) }
				c.mu.Unlock()
				if lost && onLost != nil { onLost(lease.BotID) }
			}
		}
	}
}

// ReleaseAll is used only after all updater loops have stopped.
func (c *Coordinator) ReleaseAll(ctx context.Context) error {
	leases := c.snapshot()
	var firstErr error
	for _, lease := range leases {
		if _, err := c.store.Release(ctx, lease); err != nil && firstErr == nil { firstErr = err }
		c.mu.Lock()
		current, ok := c.held[lease.BotID]
		if ok && current.FencingToken == lease.FencingToken { delete(c.held, lease.BotID) }
		c.mu.Unlock()
	}
	return firstErr
}
