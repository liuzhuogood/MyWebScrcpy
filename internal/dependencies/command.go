package dependencies

import (
	"context"
	"os/exec"
	"sync"
)

// ManagedADB is an explicit constructor target. Tests and callers passing a
// concrete executable continue to use that executable without host configuration.
const ManagedADB = "@mywebscrcpy/adb"

type Cmd struct {
	*exec.Cmd
	managed bool
	release func()
	once    sync.Once
}

func Command(name string, args ...string) *Cmd {
	return CommandContext(context.Background(), name, args...)
}
func CommandContext(ctx context.Context, name string, args ...string) *Cmd {
	return &Cmd{Cmd: exec.CommandContext(ctx, name, args...), managed: name == ManagedADB}
}

// Acquire also supports libraries (PTY) that start the underlying exec.Cmd.
func (c *Cmd) Acquire() error {
	if !c.managed {
		return nil
	}
	if active == nil {
		return exec.ErrNotFound
	}
	release, err := active.acquire("adb")
	if err != nil {
		return err
	}
	c.release = release
	path, err := active.resolveADB(active.Settings().ADB)
	if err != nil {
		c.Release()
		return err
	}
	c.Path = path
	c.Args[0] = path
	c.Err = nil
	return nil
}
func (c *Cmd) Release() {
	c.once.Do(func() {
		if c.release != nil {
			c.release()
		}
	})
}
func (c *Cmd) Start() error {
	if e := c.Acquire(); e != nil {
		return e
	}
	if e := c.Cmd.Start(); e != nil {
		c.Release()
		return e
	}
	return nil
}
func (c *Cmd) Wait() error { defer c.Release(); return c.Cmd.Wait() }
func (c *Cmd) Run() error {
	if e := c.Acquire(); e != nil {
		return e
	}
	defer c.Release()
	return c.Cmd.Run()
}
func (c *Cmd) Output() ([]byte, error) {
	if e := c.Acquire(); e != nil {
		return nil, e
	}
	defer c.Release()
	return c.Cmd.Output()
}
func (c *Cmd) CombinedOutput() ([]byte, error) {
	if e := c.Acquire(); e != nil {
		return nil, e
	}
	defer c.Release()
	return c.Cmd.CombinedOutput()
}
