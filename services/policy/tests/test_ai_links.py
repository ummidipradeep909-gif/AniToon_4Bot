import unittest
import json

import httpx

from app.ai import analyze_message


class LinkAnalysisTests(unittest.IsolatedAsyncioTestCase):
    async def test_link_feature_uses_phishing_prompt_and_accepts_phishing(self):
        async def handler(request):
            body = json.loads(request.read())
            self.assertIn("assess possible phishing links", body["messages"][0]["content"])
            self.assertIn("secure-login.example", body["messages"][1]["content"])
            self.assertNotIn("https://", body["messages"][1]["content"])
            return httpx.Response(200, json={"choices": [{"message": {"content":
                '{"category":"phishing","confidence":0.91,"recommendation":"admin_review",'
                '"rationale":"The message imitates a login flow on an unrelated host."}'}}]})

        async with httpx.AsyncClient(transport=httpx.MockTransport(handler),
                                     base_url="https://api.openai.com") as client:
            result = await analyze_message(
                "Extracted link hostnames: secure-login.example. Message context: verify account at [link]",
                api_key="test-key", model="test-model", feature="link_protection", client=client,
            )
        self.assertEqual(result.category, "phishing")
        self.assertEqual(result.recommendation, "admin_review")

    async def test_link_feature_rejects_spam_category(self):
        async def handler(_request):
            return httpx.Response(200, json={"choices": [{"message": {"content":
                '{"category":"spam","confidence":0.9,"recommendation":"admin_review",'
                '"rationale":"not a phishing assessment"}'}}]})

        async with httpx.AsyncClient(transport=httpx.MockTransport(handler),
                                     base_url="https://api.openai.com") as client:
            with self.assertRaises(RuntimeError):
                await analyze_message("hostname: example.com", api_key="test-key",
                                      model="test-model", feature="link_protection", client=client)


if __name__ == "__main__":
    unittest.main()
