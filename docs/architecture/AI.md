# AI assistance foundation

AniToon_4Bot uses a shared OpenAI analysis service in **assist** mode. AI outputs
recommendations only; it never executes Telegram moderation actions.

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

## Integrated features

- **Anti-flood:** after the configured flood threshold is crossed, AI may flag the
  triggering message for admin review. Existing flood actions remain authoritative.
- **Anti-raid:** AI reviews aggregate join-wave counts and timing. It receives no member
  IDs or message content; the existing raid threshold and quarantine remain authoritative.
- **Link protection:** deterministic allowlist and invite-link rules run first. For
  blocked links, AI reviews extracted hostnames and a message snippet with URLs redacted.
  It never fetches URLs, and its phishing warning is advisory only.

Redis cooldowns limit repeated requests. AI timeouts and provider failures must not
disable the existing deterministic moderation behaviour.

## API and safety

`POST /v1/ai/analyze` accepts message metadata and up to 4000 characters of text.
Only analysis content is sent to OpenAI; Telegram identifiers are not sent to the
provider. The response is schema-validated and constrained to `allow` or
`admin_review`; the policy service never calls Telegram. The Go client rejects
unexpected enforcement modes and punitive recommendations.

Avoid submitting sensitive/private information. Before expanding AI analysis further,
add group-level opt-in and explicit retention controls.
