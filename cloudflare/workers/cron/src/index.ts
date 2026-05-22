/**
 * Cloudflare Worker — Cron Trigger
 *
 * Fires scheduled jobs to the Go bot backend via webhook.
 * Cron schedule (UTC):
 *   - Every 15 min: fetch breaking news + market alerts
 *   - 00:00 UTC (07:00 WIB): daily market pulse + morning briefing
 *   - 11:00 UTC (18:00 WIB): evening recap
 */

export interface Env {
  BOT_WEBHOOK_URL: string;
  BOT_WEBHOOK_SECRET?: string;
}

interface CronPayload {
  job: string;
  source: string;
  timestamp: string;
}

export default {
  async scheduled(
    controller: ScheduledController,
    env: Env,
    ctx: ExecutionContext
  ): Promise<void> {
    const jobs = getJobsForCron(controller.cron);

    const promises = jobs.map((job) =>
      triggerWebhook(env, {
        job,
        source: "cloudflare_cron",
        timestamp: new Date().toISOString(),
      })
    );

    ctx.waitUntil(Promise.allSettled(promises));
  },
};

function getJobsForCron(cron: string): string[] {
  switch (cron) {
    case "*/15 * * * *":
      return ["fetch_breaking_news", "fetch_market_alerts"];
    case "0 0 * * *":
      return ["daily_market_pulse", "morning_briefing"];
    case "0 11 * * *":
      return ["evening_recap"];
    default:
      return ["fetch_breaking_news"];
  }
}

async function triggerWebhook(env: Env, payload: CronPayload): Promise<void> {
  const url = `${env.BOT_WEBHOOK_URL}/api/webhook/cron`;

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "User-Agent": "cloudflare-worker-cron/1.0",
  };

  if (env.BOT_WEBHOOK_SECRET) {
    headers["X-Webhook-Secret"] = env.BOT_WEBHOOK_SECRET;
  }

  const resp = await fetch(url, {
    method: "POST",
    headers,
    body: JSON.stringify(payload),
  });

  if (!resp.ok) {
    console.error(
      `Webhook failed for job ${payload.job}: ${resp.status} ${resp.statusText}`
    );
  }
}
