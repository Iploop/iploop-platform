"""
IPLoop Python Client
"""

import requests
from typing import Optional, Dict, Any, Union
from urllib.parse import urljoin
import base64


class IPLoopClient:
    """
    IPLoop Proxy Client
    
    Usage:
        from iploop import IPLoopClient
        
        client = IPLoopClient(api_key="your_api_key")
        
        # Simple request
        response = client.get("https://example.com")
        
        # With country targeting
        response = client.get("https://example.com", country="US")
        
        # With session (sticky IP)
        response = client.get("https://example.com", session="my_session")
        
        # Using as requests proxy
        proxies = client.get_proxy(country="DE")
        response = requests.get("https://example.com", proxies=proxies)
    """
    
    DEFAULT_PROXY_HOST = "proxy.iploop.io"
    DEFAULT_HTTP_PORT = 7777
    DEFAULT_SOCKS_PORT = 1080
    DEFAULT_API_URL = "https://api.iploop.io"
    
    def __init__(
        self,
        api_key: str,
        proxy_host: Optional[str] = None,
        http_port: Optional[int] = None,
        socks_port: Optional[int] = None,
        api_url: Optional[str] = None,
        timeout: int = 30,
    ):
        """
        Initialize IPLoop client.
        
        Args:
            api_key: Your IPLoop API key
            proxy_host: Proxy server hostname (default: proxy.iploop.io)
            http_port: HTTP proxy port (default: 7777)
            socks_port: SOCKS5 proxy port (default: 1080)
            api_url: API base URL (default: https://api.iploop.io)
            timeout: Default request timeout in seconds
        """
        self.api_key = api_key
        self.proxy_host = proxy_host or self.DEFAULT_PROXY_HOST
        self.http_port = http_port or self.DEFAULT_HTTP_PORT
        self.socks_port = socks_port or self.DEFAULT_SOCKS_PORT
        self.api_url = api_url or self.DEFAULT_API_URL
        self.timeout = timeout
        
        self._session = requests.Session()
    
    def get_proxy(
        self,
        country: Optional[str] = None,
        city: Optional[str] = None,
        session: Optional[str] = None,
        protocol: str = "http",
    ) -> Dict[str, str]:
        """
        Get proxy configuration for use with requests library.
        
        Args:
            country: Target country code (e.g., "US", "DE")
            city: Target city name
            session: Session ID for sticky IP
            protocol: "http" or "socks5"
            
        Returns:
            Dict with http and https proxy URLs
        """
        username = self._build_username(country, city, session)
        
        if protocol == "socks5":
            proxy_url = f"socks5://{username}:{self.api_key}@{self.proxy_host}:{self.socks_port}"
        else:
            proxy_url = f"http://{username}:{self.api_key}@{self.proxy_host}:{self.http_port}"
        
        return {
            "http": proxy_url,
            "https": proxy_url,
        }
    
    def get(
        self,
        url: str,
        country: Optional[str] = None,
        city: Optional[str] = None,
        session: Optional[str] = None,
        **kwargs
    ) -> requests.Response:
        """
        Make a GET request through the proxy.
        
        Args:
            url: Target URL
            country: Target country code
            city: Target city
            session: Session ID for sticky IP
            **kwargs: Additional arguments passed to requests
            
        Returns:
            requests.Response object
        """
        return self._request("GET", url, country, city, session, **kwargs)
    
    def post(
        self,
        url: str,
        country: Optional[str] = None,
        city: Optional[str] = None,
        session: Optional[str] = None,
        **kwargs
    ) -> requests.Response:
        """Make a POST request through the proxy."""
        return self._request("POST", url, country, city, session, **kwargs)
    
    def put(
        self,
        url: str,
        country: Optional[str] = None,
        city: Optional[str] = None,
        session: Optional[str] = None,
        **kwargs
    ) -> requests.Response:
        """Make a PUT request through the proxy."""
        return self._request("PUT", url, country, city, session, **kwargs)
    
    def delete(
        self,
        url: str,
        country: Optional[str] = None,
        city: Optional[str] = None,
        session: Optional[str] = None,
        **kwargs
    ) -> requests.Response:
        """Make a DELETE request through the proxy."""
        return self._request("DELETE", url, country, city, session, **kwargs)
    
    def _request(
        self,
        method: str,
        url: str,
        country: Optional[str] = None,
        city: Optional[str] = None,
        session: Optional[str] = None,
        **kwargs
    ) -> requests.Response:
        """Internal method to make requests through proxy."""
        proxies = self.get_proxy(country, city, session)
        kwargs.setdefault("timeout", self.timeout)
        kwargs.setdefault("proxies", proxies)
        
        return self._session.request(method, url, **kwargs)
    
    def _build_username(
        self,
        country: Optional[str] = None,
        city: Optional[str] = None,
        session: Optional[str] = None,
    ) -> str:
        """Build proxy username with targeting options."""
        parts = ["user"]
        
        if country:
            parts.append(f"country-{country.upper()}")
        if city:
            parts.append(f"city-{city.lower()}")
        if session:
            parts.append(f"session-{session}")
        
        return "-".join(parts)
    
    # API Methods
    
    def get_usage(self) -> Dict[str, Any]:
        """Get current usage statistics."""
        return self._api_request("GET", "/usage/summary")
    
    def get_usage_daily(self, days: int = 30) -> Dict[str, Any]:
        """Get daily usage breakdown."""
        return self._api_request("GET", f"/usage/daily?days={days}")
    
    def list_api_keys(self) -> Dict[str, Any]:
        """List all API keys."""
        return self._api_request("GET", "/keys")
    
    def create_api_key(self, name: str) -> Dict[str, Any]:
        """Create a new API key."""
        return self._api_request("POST", "/keys", json={"name": name})
    
    def delete_api_key(self, key_id: str) -> Dict[str, Any]:
        """Delete an API key."""
        return self._api_request("DELETE", f"/keys/{key_id}")
    
    def get_subscription(self) -> Dict[str, Any]:
        """Get current subscription details."""
        return self._api_request("GET", "/subscription")
    
    def _api_request(
        self,
        method: str,
        endpoint: str,
        **kwargs
    ) -> Dict[str, Any]:
        """Make an API request."""
        url = urljoin(self.api_url, endpoint)
        headers = kwargs.pop("headers", {})
        headers["Authorization"] = f"Bearer {self.api_key}"
        headers["Content-Type"] = "application/json"
        
        response = requests.request(
            method,
            url,
            headers=headers,
            timeout=self.timeout,
            **kwargs
        )
        
        response.raise_for_status()
        return response.json()


class AsyncIPLoopClient:
    """
    Async IPLoop Proxy Client using aiohttp
    
    Usage:
        import asyncio
        from iploop import AsyncIPLoopClient
        
        async def main():
            client = AsyncIPLoopClient(api_key="your_api_key")
            response = await client.get("https://example.com", country="US")
            print(await response.text())
            
        asyncio.run(main())
    """
    
    def __init__(
        self,
        api_key: str,
        proxy_host: Optional[str] = None,
        http_port: Optional[int] = None,
        timeout: int = 30,
    ):
        self.api_key = api_key
        self.proxy_host = proxy_host or IPLoopClient.DEFAULT_PROXY_HOST
        self.http_port = http_port or IPLoopClient.DEFAULT_HTTP_PORT
        self.timeout = timeout
        self._session = None
    
    async def __aenter__(self):
        import aiohttp
        self._session = aiohttp.ClientSession()
        return self
    
    async def __aexit__(self, *args):
        if self._session:
            await self._session.close()
    
    def _get_proxy_url(
        self,
        country: Optional[str] = None,
        city: Optional[str] = None,
        session: Optional[str] = None,
    ) -> str:
        parts = ["user"]
        if country:
            parts.append(f"country-{country.upper()}")
        if city:
            parts.append(f"city-{city.lower()}")
        if session:
            parts.append(f"session-{session}")
        
        username = "-".join(parts)
        return f"http://{username}:{self.api_key}@{self.proxy_host}:{self.http_port}"
    
    async def get(
        self,
        url: str,
        country: Optional[str] = None,
        city: Optional[str] = None,
        session: Optional[str] = None,
        **kwargs
    ):
        """Make async GET request through proxy."""
        proxy = self._get_proxy_url(country, city, session)
        return await self._session.get(url, proxy=proxy, timeout=self.timeout, **kwargs)
    
    async def post(
        self,
        url: str,
        country: Optional[str] = None,
        city: Optional[str] = None,
        session: Optional[str] = None,
        **kwargs
    ):
        """Make async POST request through proxy."""
        proxy = self._get_proxy_url(country, city, session)
        return await self._session.post(url, proxy=proxy, timeout=self.timeout, **kwargs)
