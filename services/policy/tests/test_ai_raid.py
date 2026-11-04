import unittest

import httpx

from app.ai import analyze_message


class RaidAnalysisTests(unittest.IsolatedAsyncioTestCase):
    async def test_raid_feature_uses_telemetry_prompt_and_accepts_raid(self):
        async def handler(request):
            payload = request.read()
            # Request payload is JSON encoded by httpx; inspect it without making a network call.
            import json
            body = json.loads(payload)
            self.assertIn("join-wave telemetry", body["messages"][0]["content"])
            self.assertIn("join wave reached", body["messages"][1]["content"])
            return httpx.Response(200, json={"choices": [{"message": {"content":
                '{"category":"raid","confidence":0.88,"recommendation":"admin_review",'
                '"rationale":"The join count rapidly reached the configured threshold."}'}}]})

        async with httpx.AsyncClient(transport=httpx.MockTransport(handler),
                                     base_url="https://api.openai.com") as client:
            result = await analyze_message(
                "A group join wave reached 8 joins in 10 seconds.",
                api_key="test-key", model="test-model", feature="anti_raid", client=client,
            )
        self.assertEqual(result.category, "raid")
        self.assertEqual(result.recommendation, "admin_review")
        self.assertEqual(result.confidence, 0.88)

    async def test_raid_feature_rejects_unrelated_message_category(self):
        async def handler(_request):
            return httpx.Response(200, json={"choices": [{"message": {"content":
                '{"category":"phishing","confidence":0.9,"recommendation":"admin_review",'
                '"rationale":"not a raid assessment"}'}}]})

        async with httpx.AsyncClient(transport=httpx.MockTransport(handler),
                                     base_url="https://api.openai.com") as client:
            with self.assertRaises(RuntimeError):
                await analyze_message("join wave", api_key="test-key",
                                      model="test-model", feature="anti_raid", client=client)


if __name__ == "__main__":
    unittest.main()
