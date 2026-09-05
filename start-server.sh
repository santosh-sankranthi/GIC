#!/usr/bin/env bash
set -e

# Load from .env if present
if [ -f .env ]; then
  export $(grep -v '^#' .env | xargs)
fi

# feelc server launch script with OpenRouter configuration
export FEELC_LLM_PROVIDER="${FEELC_LLM_PROVIDER:-openrouter}"
export FEELC_LLM_BASE_URL="${FEELC_LLM_BASE_URL:-https://openrouter.ai/api}"
export FEELC_LLM_MODEL="${FEELC_LLM_MODEL:-nvidia/nemotron-3-ultra-550b-a55b:free}"
export FEELC_LLM_API_KEY="${FEELC_LLM_API_KEY:-$OPENROUTER_API_KEY}"

echo "Starting feelc serve on http://localhost:8080 ..."
echo "LLM configured: $FEELC_LLM_PROVIDER ($FEELC_LLM_MODEL)"

./feelc serve --project sample-project --addr :8080 --ui --allow-edit --watch
