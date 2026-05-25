package main

import (
	"context"
	"log"
	"time"
)

func (srv *server) startScheduler(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	log.Println("scheduler: started")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			srv.runDueSchedules(ctx)
		}
	}
}

func (srv *server) runDueSchedules(ctx context.Context) {
	schedules, err := srv.getDueSchedules(ctx)
	if err != nil {
		log.Printf("scheduler: getDueSchedules: %v", err)
		return
	}
	for _, s := range schedules {
		// Dedup: skip if a job is already running or pending for this connection
		var runningCount int
		if err := srv.db.QueryRow(ctx,
			`SELECT COUNT(*) FROM audit_jobs WHERE connection_id=$1 AND status IN ('pending','running')`,
			s.ConnectionID).Scan(&runningCount); err == nil && runningCount > 0 {
			log.Printf("scheduler: skipping schedule %s — job already running for connection %s", s.ID, s.ConnectionID)
			continue
		}

		log.Printf("scheduler: triggering schedule %s (connection=%s interval=%s)", s.ID, s.ConnectionID, s.Interval)
		conn, err := srv.getConnection(ctx, s.ConnectionID)
		if err != nil {
			log.Printf("scheduler: getConnection: %v", err)
			continue
		}
		job, err := srv.createJob(ctx, s.ConnectionID, s.UserID, s.TenantID, conn.ConnType)
		if err != nil {
			log.Printf("scheduler: createJob: %v", err)
			continue
		}
		if err := srv.advanceSchedule(ctx, s.ID, s.Interval); err != nil {
			log.Printf("scheduler: advanceSchedule: %v", err)
		}
		connType := conn.ConnType
		// Use the scheduler's context so goroutines stop on server shutdown
		jobCtx := ctx
		go func(jobID, connID, userID string) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("scheduler: audit panic %s: %v", jobID, rec)
					srv.updateJobFailed(jobCtx, jobID, "internal error")
				}
			}()
			if connType == "code" {
				srv.runCodeAudit(jobID, connID, userID)
			} else {
				srv.runAudit(jobID, connID, userID)
			}
		}(job.ID, s.ConnectionID, s.UserID)
	}
}
