# Token Validation Commands

After writing config.env, validate each enabled platform's credentials to catch typos early.

## Telegram

```bash
source ~/.tlive/config.env
curl -s "https://api.telegram.org/bot${TL_TG_BOT_TOKEN}/getMe"
```
Expected: response contains `"ok":true`. If not, the Bot Token is invalid — re-check with @BotFather.

## Discord

Verify token format matches: `[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`

```bash
source ~/.tlive/config.env
echo "$TL_DC_BOT_TOKEN" | grep -qP '^[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$' && echo "Format OK" || echo "Format INVALID"
```

A format mismatch means the token was copied incorrectly from the Discord Developer Portal.

## Feishu / Lark

```bash
source ~/.tlive/config.env
curl -s -X POST "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal" \
  -H "Content-Type: application/json" \
  -d "{\"app_id\":\"${TL_FS_APP_ID}\",\"app_secret\":\"${TL_FS_APP_SECRET}\"}"
```
Expected: response contains `"code":0`. If not, check App ID and App Secret in the Feishu Developer Console.
