package api

import (
	"context"
	"fmt"
	"time"
)

// GetRecentTasks returns the most recent tasks for a node.
func (c *Client) GetRecentTasks(ctx context.Context, node string) ([]Task, error) {
	var out APIResponse[[]Task]
	if err := c.get(ctx, fmt.Sprintf("/nodes/%s/tasks", node), &out); err != nil {
		return nil, fmt.Errorf("get tasks: %w", err)
	}
	return out.Data, nil
}

// GetTaskStatus returns the current status/exitstatus of a task by UPID.
func (c *Client) GetTaskStatus(ctx context.Context, node, upid string) (*TaskStatusResponse, error) {
	var out APIResponse[TaskStatusResponse]
	if err := c.get(ctx, fmt.Sprintf("/nodes/%s/tasks/%s/status", node, upid), &out); err != nil {
		return nil, fmt.Errorf("get task status: %w", err)
	}
	return &out.Data, nil
}

// GetTaskLog fetches a page of task log lines.
func (c *Client) GetTaskLog(ctx context.Context, node, upid string, start, limit int) ([]TaskLog, error) {
	var out APIResponse[[]TaskLog]
	path := fmt.Sprintf("/nodes/%s/tasks/%s/log?start=%d&limit=%d", node, upid, start, limit)
	if err := c.get(ctx, path, &out); err != nil {
		return nil, fmt.Errorf("get task log: %w", err)
	}
	return out.Data, nil
}

// WatchTask streams task log lines into ch until the task completes.
// Call in a goroutine. ch is closed when done.
func (c *Client) WatchTask(ctx context.Context, node, upid string, ch chan<- TaskLog) {
	defer close(ch)
	linesSeen := 0
	pollInterval := 500 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}

		// Fetch new log lines
		lines, err := c.GetTaskLog(ctx, node, upid, linesSeen, 50)
		if err == nil {
			for _, l := range lines {
				select {
				case ch <- l:
				case <-ctx.Done():
					return
				}
			}
			linesSeen += len(lines)
		}

		// Check if task is done
		status, err := c.GetTaskStatus(ctx, node, upid)
		if err != nil {
			continue
		}
		if status.Status == "stopped" {
			return
		}
	}
}
