package testenv

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
)

// A standard-library fake driver exercises setup and session lifetime without PG.
type suiteConnector struct{ conn *suiteConn }

func (c suiteConnector) Connect(context.Context) (driver.Conn, error) { return c.conn, nil }
func (c suiteConnector) Driver() driver.Driver                        { return suiteDriver{} }

type suiteDriver struct{}

func (suiteDriver) Open(string) (driver.Conn, error) { return nil, errors.New("unused") }

type suiteConn struct {
	pingErr, lockErr, unlockErr error
	locked, unlocked, closed    bool
}

func (c *suiteConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unused") }
func (c *suiteConn) Begin() (driver.Tx, error)           { return nil, errors.New("unused") }
func (c *suiteConn) Close() error                        { c.closed = true; return nil }
func (c *suiteConn) Ping(context.Context) error          { return c.pingErr }
func (c *suiteConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	if strings.Contains(query, "pg_advisory_unlock") {
		c.unlocked = true
		c.locked = false
		return driver.RowsAffected(1), c.unlockErr
	}
	if c.lockErr != nil {
		return nil, c.lockErr
	}
	c.locked = true
	return driver.RowsAffected(1), nil
}

func TestPostgresSuiteFailsClosedAndPinsLock(t *testing.T) {
	for _, phase := range []string{"success", "ping", "lock", "unlock"} {
		t.Run(phase, func(t *testing.T) {
			conn := &suiteConn{}
			failure := errors.New("fixture failure")
			switch phase {
			case "ping":
				conn.pingErr = failure
			case "lock":
				conn.lockErr = failure
			case "unlock":
				conn.unlockErr = failure
			}
			pool := sql.OpenDB(suiteConnector{conn})
			defer pool.Close()
			called := false
			code := withPostgresLock(pool, func() int {
				called = true
				if !conn.locked {
					t.Fatal("runner started without lock")
				}
				pool.SetMaxIdleConns(0) // Must not close the pinned lock session.
				if conn.closed {
					t.Fatal("lock session was returned to the pool before runner finished")
				}
				return 0
			})
			if called != (phase == "success" || phase == "unlock") {
				t.Fatal("suite ran after setup failure")
			}
			if (code == 0) != (phase == "success") {
				t.Fatalf("unexpected suite result %d", code)
			}
			if called && !conn.unlocked {
				t.Fatal("lock not released on the pinned connection")
			}
		})
	}
}
