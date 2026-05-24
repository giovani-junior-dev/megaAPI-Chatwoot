# AlertManager configuration

This file (`alertmanager.yml`) ships with two receivers and two placeholders
you MUST replace before bringing the stack up:

| Placeholder    | Where it lives                  | Replace with                                         |
|----------------|---------------------------------|------------------------------------------------------|
| `WEBHOOK_URL`  | `receivers[default-webhook]`    | Full HTTPS webhook URL (Slack, Discord, generic).    |
| `EMAIL_TO`     | `receivers[critical-email]`     | Comma-separated list of on-call recipient addresses. |

You also have to populate the `global.smtp_*` fields if you use the email
receiver — leaving them as placeholders is fine if you only intend to route
through the webhook.

## `webhook_configs` vs `email_configs`

Both blocks are AlertManager receiver types. They are NOT interchangeable —
pick the one that matches your transport.

### `webhook_configs`

- Use for: Slack, Discord, Mattermost, PagerDuty events API, generic HTTPS
  POST to any service that accepts AlertManager's JSON payload.
- AlertManager POSTs a JSON body describing the firing alerts; the receiving
  service is expected to parse and render it.
- Slack and Discord both accept an incoming-webhook URL out of the box; for
  richer formatting use `slack_configs` / a dedicated bridge instead.
- Recommended default for chat-first teams: one webhook URL, group by
  `alertname` + `service`, `repeat_interval: 4h`.

### `email_configs`

- Use for: regulated environments, after-hours pager fallback, durable record
  of incidents that survives chat retention.
- Requires the `global.smtp_*` block to be filled out (host, port, from,
  credentials). Without it the receiver errors on every send.
- Higher latency and noisier than chat — reserve for `severity = critical`.

## Combining receivers

The shipped `route` sends everything to the webhook AND, when an alert has
`severity = "critical"`, ALSO emails on-call (`continue: true` on the
sub-route fires both branches). Adjust matchers in `route.routes` to add
PagerDuty, OpsGenie, or per-tenant routing.

## Validating changes

```bash
docker run --rm -v "$PWD/alertmanager.yml:/etc/alertmanager/alertmanager.yml" \
  prom/alertmanager:v0.27.0 amtool check-config /etc/alertmanager/alertmanager.yml
```

`amtool` rejects unresolved placeholders for SMTP credentials only — webhook
URL and email address are stringly-typed and validated at send time.
