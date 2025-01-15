import logging
from flask import Flask, request, jsonify

app = Flask(__name__)

# Configure logging
logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger(__name__)


@app.route("/", methods=["GET"])
def handle_get():
    return ("OK", 200)


@app.route("/", methods=["POST"])
def handle_post():
    try:
        data = request.get_json()
        if not data:
            return (jsonify({"error": "No JSON payload provided"}), 400)

        foo_value = data.get("foo")
        logger.info(f"Received foo: {foo_value}")
        return ("OK", 200)
    except Exception as e:
        return (
            jsonify({"error": str(e)}),
            500,
        )


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8080)
