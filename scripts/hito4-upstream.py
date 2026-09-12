#!/usr/bin/env python3
"""Deterministic local OpenAI-compatible fixture for the Milestone 4 gate."""

import json
import sys
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, format, *args):
        return

    def do_POST(self):
        try:
            length = int(self.headers.get("Content-Length", "0"))
            request = json.loads(self.rfile.read(length))
        except (ValueError, json.JSONDecodeError):
            self.send_error(400)
            return
        if (
            self.path != "/v1/chat/completions"
            or self.headers.get("Authorization")
            != "Bearer benchmark-provider-secret"
            or request.get("model") != "example-chat-model"
        ):
            self.send_error(400)
            return
        if request.get("stream"):
            self._stream()
            return
        self._completion()

    def _completion(self):
        response = {
            "id": "bench",
            "object": "chat.completion",
            "model": "example-chat-model",
            "choices": [
                {
                    "index": 0,
                    "message": {
                        "role": "assistant",
                        "content": "benchmark-ok",
                    },
                    "finish_reason": "stop",
                }
            ],
        }
        self._write_json(response)

    def _stream(self):
        frames = [
            {
                "id": "bench",
                "object": "chat.completion.chunk",
                "model": "example-chat-model",
                "choices": [
                    {
                        "index": 0,
                        "delta": {"role": "assistant"},
                        "finish_reason": None,
                    }
                ],
            },
            {
                "id": "bench",
                "object": "chat.completion.chunk",
                "model": "example-chat-model",
                "choices": [
                    {
                        "index": 0,
                        "delta": {"content": "benchmark-ok"},
                        "finish_reason": None,
                    }
                ],
            },
            {
                "id": "bench",
                "object": "chat.completion.chunk",
                "model": "example-chat-model",
                "choices": [
                    {"index": 0, "delta": {}, "finish_reason": "stop"}
                ],
            },
        ]
        body = "".join(
            "data: " + json.dumps(frame, separators=(",", ":")) + "\n\n"
            for frame in frames
        )
        encoded = (body + "data: [DONE]\n\n").encode()
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        midpoint = len(encoded) // 2
        self.wfile.write(encoded[:midpoint])
        self.wfile.flush()
        time.sleep(0.05)
        self.wfile.write(encoded[midpoint:])
        self.wfile.flush()

    def _write_json(self, value):
        encoded = json.dumps(value, separators=(",", ":")).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)


def main():
    if len(sys.argv) != 2:
        raise SystemExit("usage: hito4-upstream.py PORT")
    ThreadingHTTPServer(("127.0.0.1", int(sys.argv[1])), Handler).serve_forever()


if __name__ == "__main__":
    main()
