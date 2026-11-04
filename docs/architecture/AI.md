# AI assistance foundation and first integration

AniToon_4Bot uses a shared OpenAI analysis service. The first live feature integration is
anti-flood assist mode. The existing flood threshold and administrator-selected action
remain authoritative; AI can only post a review suggestion in the group.

## Configuration

Configure the Go bot and Python policy service with the same private values:

- `POLICY_SERVICE_URL`: private URL of the policy service, e.g. `http://policy:8000`
- `AI_SERVICE_TOKEN`: long random shared secret, set on both Go and policy service
- `AI_ENABLED=true`: enables provider requests in the policy service
- `OPENAI_API_KEY`: OpenAI API key, stored only as a deployment secret
- `OPENAI_MODEL=gpt-4o-mini`
- `AI_TIMEOUT_SECONDS=12`

Generate a service token with `openssl rand -hex 32`. Do not commit either secret.
AI remains disabled unless explicitly enabled and both secrets are configured.
The endpoint rejects calls without the service-token header.

## Anti-flood assist behaviour

When an already-configured anti-flood threshold is exceeded, the Go handler can submit
the triggering message text to the shared policy service. A Redis cooldown limits AI
review requests to one per user/group/bot per minute. If the result recommends
`admin_review`, the bot posts a review suggestion. The existing configured flood
action continues unchanged. AI failures/timeouts are ignored so ordinary moderation
keeps working.

## API and safety

`POST /v1/ai/analyze` accepts message metadata and up to 4000 characters of text.
Only the text and fixed system instructions are sent to OpenAI; Telegram identifiers
are not sent to the provider. The response is schema-validated and constrained to
`allow` or `admin_review`; the policy service never calls Telegram. The Go client
rejects unexpected enforcement modes and punitive recommendations.

Avoid submitting sensitive/private information. Add group-level opt-in and explicit
retention controls before expanding AI analysis to other message features.
