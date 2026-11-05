import unittest

from fastapi import HTTPException

from app.security import require_service_token


class ServiceTokenTests(unittest.TestCase):
    def test_matching_token_is_accepted(self):
        self.assertIsNone(require_service_token("shared-secret", "shared-secret"))

    def test_missing_or_incorrect_candidate_is_rejected(self):
        for candidate in ("", "wrong"):
            with self.subTest(candidate=candidate):
                with self.assertRaises(HTTPException) as raised:
                    require_service_token(candidate, "shared-secret")
                self.assertEqual(raised.exception.status_code, 401)
                self.assertEqual(raised.exception.detail, "unauthorized")

    def test_missing_configured_token_fails_closed(self):
        with self.assertRaises(HTTPException) as raised:
            require_service_token("anything", "")
        self.assertEqual(raised.exception.status_code, 401)
        self.assertEqual(raised.exception.detail, "unauthorized")

    def test_no_secret_value_is_returned_in_error(self):
        with self.assertRaises(HTTPException) as raised:
            require_service_token("wrong-candidate", "very-sensitive-shared-token")
        self.assertNotIn("very-sensitive-shared-token", str(raised.exception.detail))
        self.assertNotIn("wrong-candidate", str(raised.exception.detail))


if __name__ == "__main__":
    unittest.main()
