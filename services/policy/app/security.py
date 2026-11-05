"""Authentication helpers for internal policy-service endpoints."""
import secrets
from fastapi import HTTPException


def require_service_token(candidate: str, configured: str) -> None:
    """Require a configured shared token and compare it in constant time."""
    if not configured or not secrets.compare_digest(candidate, configured):
        raise HTTPException(status_code=401, detail="unauthorized")
