"""Integration tests for API endpoints."""

import pytest
import requests
import time
import subprocess
import threading
import os
from pathlib import Path


class TestAPIEndpointsIntegration:
    """Integration tests for API endpoints using real containers."""

    @pytest.fixture(scope="class")
    def api_server(self, db_config, redis_config):
        """Start API server for testing."""
        # Set environment variables
        env = os.environ.copy()
        env.update({
            "POSTGRES_HOST": db_config["host"],
            "POSTGRES_PORT": str(db_config["port"]),
            "POSTGRES_USER": db_config["user"],
            "POSTGRES_PASSWORD": db_config["password"],
            "POSTGRES_DB": db_config["database"],
            "REDIS_HOST": redis_config["host"],
            "REDIS_PORT": str(redis_config["port"]),
            "API_HOST": "127.0.0.1",
            "API_PORT": "8888",
            "LOG_FORMAT": "json"
        })

        # Start API server
        server_process = subprocess.Popen(
            ["python", "-m", "uvicorn", "src.monitoring.api:app", "--host", "127.0.0.1", "--port", "8888"],
            cwd=Path(__file__).parent.parent.parent,
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE
        )

        # Wait for server to start
        time.sleep(3)

        yield "http://127.0.0.1:8888"

        # Cleanup
        server_process.terminate()
        server_process.wait()

    def test_health_endpoint(self, api_server):
        """Test health endpoint."""
        response = requests.get(f"{api_server}/health", timeout=5)

        assert response.status_code == 200
        data = response.json()

        assert data["status"] in ["healthy", "degraded"]
        assert "timestamp" in data
        assert "checks" in data
        assert "database" in data["checks"]
        assert "redis" in data["checks"]
        assert "nltk" in data["checks"]

    def test_metrics_endpoint(self, api_server):
        """Test metrics endpoint."""
        response = requests.get(f"{api_server}/metrics", timeout=5)

        assert response.status_code == 200
        content_type = response.headers.get("content-type", "")
        assert "text/plain" in content_type

        # Check that response contains expected metrics
        content = response.text
        assert "news_analyzer_version" in content
        assert "analysis_duration_seconds" in content
        assert "clustering_quality_silhouette" in content

    def test_status_endpoint(self, api_server):
        """Test detailed status endpoint."""
        response = requests.get(f"{api_server}/status", timeout=5)

        assert response.status_code == 200
        data = response.json()

        required_fields = [
            "service", "version", "status", "components",
            "system", "configuration", "timestamp"
        ]

        for field in required_fields:
            assert field in data

        assert data["service"] == "news-analyzer"
        assert "database" in data["components"]
        assert "redis" in data["components"]

        # Check system metrics
        system = data["system"]
        assert "memory_used_percent" in system
        assert "disk_used_percent" in system
        assert "cpu_count" in system

    def test_diagnostics_endpoint(self, api_server):
        """Test diagnostics endpoint."""
        response = requests.get(f"{api_server}/diagnostics", timeout=10)

        assert response.status_code == 200
        data = response.json()

        assert "timestamp" in data
        assert "diagnostics" in data

        diagnostics = data["diagnostics"]

        # Check that all expected components are diagnosed
        expected_components = ["database", "redis", "nltk", "ml"]
        for component in expected_components:
            assert component in diagnostics
            assert "status" in diagnostics[component]

    def test_analyze_endpoint_disabled(self, api_server):
        """Test that analyze endpoint returns appropriate response."""
        # This test assumes the analyze endpoint is not fully implemented
        # or requires additional setup
        response = requests.post(f"{api_server}/analyze", timeout=5)

        # Should not fail with 404
        assert response.status_code in [200, 400, 500]  # Various possible responses

    def test_root_endpoint(self, api_server):
        """Test root endpoint."""
        response = requests.get(f"{api_server}/", timeout=5)

        assert response.status_code == 200
        data = response.json()

        assert "service" in data
        assert "version" in data
        assert "endpoints" in data
        assert data["service"] == "News Analyzer API"

    def test_cors_headers(self, api_server):
        """Test CORS headers."""
        response = requests.options(f"{api_server}/health", timeout=5)

        assert response.status_code == 200

        # Check CORS headers
        headers = response.headers
        assert "access-control-allow-origin" in headers
        assert "access-control-allow-methods" in headers
        assert "access-control-allow-headers" in headers

    def test_json_content_type(self, api_server):
        """Test that JSON endpoints return correct content type."""
        endpoints = ["/health", "/status", "/diagnostics", "/"]

        for endpoint in endpoints:
            response = requests.get(f"{api_server}{endpoint}", timeout=5)
            assert response.status_code == 200

            content_type = response.headers.get("content-type", "")
            assert "application/json" in content_type

    def test_error_handling(self, api_server):
        """Test error handling for invalid requests."""
        # Test invalid endpoint
        response = requests.get(f"{api_server}/nonexistent", timeout=5)
        assert response.status_code == 404

        # Test invalid method
        response = requests.put(f"{api_server}/health", timeout=5)
        assert response.status_code in [405, 404]  # Method not allowed or not found

    def test_health_during_load(self, api_server):
        """Test health endpoint under concurrent load."""
        import concurrent.futures

        def make_request():
            return requests.get(f"{api_server}/health", timeout=5)

        # Make 10 concurrent requests
        with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
            futures = [executor.submit(make_request) for _ in range(10)]
            responses = [future.result() for future in concurrent.futures.as_completed(futures)]

        # All requests should succeed
        for response in responses:
            assert response.status_code == 200
            data = response.json()
            assert "status" in data

    @pytest.mark.parametrize("endpoint", ["/health", "/status", "/metrics", "/diagnostics"])
    def test_endpoints_responsive(self, api_server, endpoint):
        """Test that all main endpoints are responsive."""
        response = requests.get(f"{api_server}{endpoint}", timeout=5)

        # Should not be 5xx errors (server errors)
        assert response.status_code < 500