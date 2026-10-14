package orden

import (
	"context"
	"time"
)

func (s *Server) RunMaintenance(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		work, cancel := context.WithTimeout(ctx, 10*time.Second)
		for _, query := range []string{
			"DELETE FROM rate_limits WHERE window_start<now()-interval '1 day'",
			"DELETE FROM sessions WHERE expires_at<=now()",
			"DELETE FROM activation_codes WHERE expires_at<=now()",
			"DELETE FROM gmail_oauth_states WHERE expires_at<=now()",
		} {
			if _, err := s.db.Exec(work, query); err != nil {
				break
			}
		}
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
