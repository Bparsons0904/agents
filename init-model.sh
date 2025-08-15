#!/bin/sh

# Start ollama in background
ollama serve &
OLLAMA_PID=$!

# Wait for ollama to be ready
echo "Waiting for Ollama to start..."
sleep 10

# Check if qwen3:14b-q4_K_M model exists
if ollama list | grep -q "qwen3:14b-q4_K_M"; then
    echo "Model qwen3:14b-q4_K_M already exists, skipping download"
else
    echo "Downloading qwen3:14b-q4_K_M model..."
    ollama pull qwen3:14b-q4_K_M
    echo "Model download completed"
fi

# Keep ollama running in foreground
wait $OLLAMA_PID