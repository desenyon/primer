from http.server import HTTPServer, BaseHTTPRequestHandler
import os


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b"Repository running.")


HTTPServer(("127.0.0.1", int(os.environ.get("PORT", "8000"))), Handler).serve_forever()
