import json
import unittest

import httpx

from app.ai import analyze_message


class BlacklistAnalysisTests(unittest.IsolatedAsyncioTestCase):
    async def test_blacklist_feature_uses_contextual_prompt_and_allows_quote(self):
        async def handler(request):
            body = json.loads(request.read())
            prompt = body["messages"][0]["content"]
            self.assertIn("harmless discussion", prompt)
            self.assertIn("spacing, punctuation", prompt)
            self.assertIn("configured keyword blacklist phrase", prompt)
            return httpx.Response(200, json={"choices": [{"message": {"content":
                '{"category":"none","confidence":0.94,"recommendation":"allow",'
                '"rationale":"The phrase is quoted in a neutral educational explanation."}'}}]})

        async with httpx.AsyncClient(transport=httpx.MockTransport(handler),
                                     base_url="https://api.openai.com") as client:
            result = await analyze_message(
                'Configured phrase: "bad phrase". Match: formatting variant. Message: discussing the phrase in class.',
                api_key="test-key", model="test-model", feature="keyword_blacklist", client=client,
            )
        self.assertEqual(result.category, "none")
        self.assertEqual(result.recommendation, "allow")

    async def test_blacklist_feature_accepts_advisory_admin_review(self):
        async def handler(_request):
            return httpx.Response(200, json={"choices": [{"message": {"content":
                '{"category":"spam","confidence":0.88,"recommendation":"admin_review",'
                '"rationale":"The spacing appears intended to evade the configured filter."}'}}]})

        async with httpx.AsyncClient(transport=httpx.MockTransport(handler),
                                     base_url="https://api.openai.com") as client:
            result = await analyze_message("obfuscated phrase", api_key="test-key",
                                           model="test-model", feature="keyword_blacklist", client=client)
        self.assertEqual(result.category, "spam")
        self.assertEqual(result.recommendation, "admin_review")

    async def test_blacklist_feature_rejects_phishing_category(self):
        async def handler(_request):
            return httpx.Response(200, json={"choices": [{"message": {"content":
                '{"category":"phishing","confidence":0.9,"recommendation":"admin_review",'
                '"rationale":"not a blacklist assessment"}'}}]})

        async with httpx.AsyncClient(transport=httpx.MockTransport(handler),
                                     base_url="https://api.openai.com") as client:
            with self.assertRaises(RuntimeError):
                await analyze_message("message", api_key="test-key", model="test-model",
                                      feature="keyword_blacklist", client=client)


if __name__ == "__main__":
    unittest.main()
