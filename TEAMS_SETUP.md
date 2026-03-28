# Microsoft Teams Integration — Setup Guide

PicoClaw supports two-way Teams integration:
- **Receive** — chat with PicoClaw inside Teams like a bot
- **Send** — PicoClaw posts messages to a Teams channel

---

## Part 1 — Receive Messages (Bot)

### Step 1 — Install ngrok
Teams needs a public HTTPS URL to reach your local machine.

1. Download ngrok: https://ngrok.com/download
2. Run: `ngrok http 18790`
3. Copy the `https://` forwarding URL e.g. `https://abc123.ngrok.io`

Keep ngrok running whenever you want Teams to work.

### Step 2 — Register a Bot

1. Go to https://dev.botframework.com/bots/new
2. Sign in with your Microsoft account
3. Fill in:
   - Display name: PicoClaw
   - Messaging endpoint: `https://abc123.ngrok.io/teams`
4. Click **Create New Microsoft App ID and password**
5. Generate and **copy the password immediately** — shown only once
6. Copy the App ID too

### Step 3 — Store Credentials

```bat
keytool.exe set teams-app-password YOUR_PASSWORD_HERE
keytool.exe set teams-webhook-url https://YOUR_ORG.webhook.office.com/...
```

### Step 4 — Update config.yaml

```yaml
teams:
  enabled: true
  app_id: "your-microsoft-app-id-here"
  allowed_users:
    - "your.name@outlook.com"
```

### Step 5 — Add Bot to Teams

1. In Bot Framework Portal → your bot → Channels → Microsoft Teams → Save
2. Click Open in Teams

### Step 6 — Restart PicoClaw

```bat
picoclaw.exe
```

You should see: `[teams] webhook endpoint: POST /teams`

---

## Part 2 — Send Messages to Teams (Outbound)

1. In Teams: channel → ... → Connectors → Incoming Webhook → Configure
2. Name it PicoClaw → Create → Copy the webhook URL
3. Store it: `keytool.exe set teams-webhook-url https://...`

---

## Troubleshooting

| Problem | Fix |
|---|---|
| Bot not responding | Check ngrok is running and endpoint URL matches |
| auth failed in logs | Re-run keytool set teams-app-password |
| no app password warning | Run keytool.exe set teams-app-password |
| Webhook URL rejected | Webhook URLs are external — only used for outbound sending |
