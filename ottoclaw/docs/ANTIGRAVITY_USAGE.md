# Using Antigravity Provider in OttoClaw

This guide explains how to set up and use the **Antigravity** (Google Cloud Code Assist) provider in OttoClaw.

## Prerequisites

1.  A Google account.
2.  Google Cloud Code Assist enabled (usually available via the "Gemini for Google Cloud" onboarding).

## 1. Authentication

To authenticate with Antigravity, run the following command:

```bash
ottoclaw auth login --provider antigravity
```

### Manual Authentication (Headless/VPS)
If you are running on a server (Coolify/Docker) and cannot reach `localhost`, follow these steps:
1.  Run the command above.
2.  Copy the URL provided and open it in your local browser.
3.  Complete the login.
4.  Your browser will redirect to a `localhost:51121` URL (which will fail to load).
5.  **Copy that final URL** from your browser's address bar.
6.  **Paste it back into the terminal** where OttoClaw is waiting.

OttoClaw will extract the authorization code and complete the process automatically.

## 2. Managing Models

### List Available Models
To see which models your project has access to and check their quotas:

```bash
ottoclaw auth models
```

### Switch Models
You can change the default model in `~/.ottoclaw/config.json` or override it via the CLI:

```bash
# Override for a single command
ottoclaw agent -m "Hello" --model claude-opus-4-6-thinking
```

## 3. Real-world Usage (Coolify/Docker)

If you are deploying via Coolify or Docker, follow these steps to test:

1.  **Environment Variables**:
    *   `OTTOCLAW_AGENTS_DEFAULTS_MODEL=gemini-flash`
2.  **Authentication persistence**: 
    If you've logged in locally, you can copy your credentials to the server:
    ```bash
    scp ~/.ottoclaw/auth.json user@your-server:~/.ottoclaw/
    ```
    *Alternatively*, run the `auth login` command once on the server if you have terminal access.

## 4. Troubleshooting

*   **Empty Response**: If a model returns an empty reply, it may be restricted for your project. Try `gemini-3-flash` or `claude-opus-4-6-thinking`.
*   **429 Rate Limit**: Antigravity has strict quotas. OttoClaw will display the "reset time" in the error message if you hit a limit.
*   **404 Not Found**: Ensure you are using a model ID from the `ottoclaw auth models` list. Use the short ID (e.g., `gemini-3-flash`) not the full path.

## 5. Summary of Working Models

Based on testing, the following models are most reliable:
*   `gemini-3-flash` (Fast, highly available)
*   `gemini-2.5-flash-lite` (Lightweight)
*   `claude-opus-4-6-thinking` (Powerful, includes reasoning)
