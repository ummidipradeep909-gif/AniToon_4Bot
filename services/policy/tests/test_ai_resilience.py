import asyncio
import unittest

import httpx

from app.ai import analyze_message


class AIResilienceTests(unittest.IsolatedAsyncioTestCase):
    async def test_provider_timeout_is_reported_as_temporary_failure(self):
        async def handler(request):
            raise httpx.ReadTimeout("provider timed out", request=request)

        async with httpx.AsyncClient(
            transport=httpx.MockTransport(handler), base_url="https://api.openai.com"
        ) as client:
            with self.assertRaises(RuntimeError):
                await analyze_message("test", api_key="test-key", model="test-model",
                                      timeout_seconds=0.001, client=client)

    async def test_provider_http_error_is_fail_safe(self):
        async def handler(_request):
            return httpx.Response(503, json={"error": "provider unavailable"})

        async with httpx.AsyncClient(
            transport=httpx.MockTransport(handler), base_url="https://api.openai.com"
        ) as client:
            with self.assertRaisesRegex(RuntimeError, "temporarily unavailable"):
                await analyze_message("test", api_key="test-key", model="test-model", client=client)

    async def test_malformed_provider_payload_is_rejected(self):
        async def handler(_request):
            return httpx.Response(200, json={"choices": []})

        async with httpx.AsyncClient(
            transport=httpx.MockTransport(handler), base_url="https://api.openai.com"
        ) as client:
            with self.assertRaisesRegex(RuntimeError, "temporarily unavailable"):
                await analyze_message("test", api_key="test-key", model="test-model", client=client)

    async def test_concurrent_ai_requests_do_not_share_request_context(self):
        active = 0
        peak = 0
        lock = asyncio.Lock()

        async def handler(request):
            nonlocal active, peak
            async with lock:
                active += 1
                peak = max(peak, active)
            await asyncio.sleep(0.01)
            async with lock:
                active -= 1
            return httpx.Response(200, json={"choices": [{"message": {"content":
                '{"category":"none","confidence":0.1,"recommendation":"allow","rationale":"ok"}'}}]})

        async with httpx.AsyncClient(
            transport=httpx.MockTransport(handler), base_url="https://api.openai.com"
        ) as client:
            results = await asyncio.gather(*[
                analyze_message(f"message {i}", api_key="test-key", model="test-model", client=client)
                for i in range(40)
            ])
        self.assertEqual(len(results), 40)
        self.assertGreater(peak, 1)


if __name__ == "__main__":
    unittest.main()
