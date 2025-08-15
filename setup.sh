#!/bin/bash
set -e

echo "🤖 Setting up AI Agent Development System"

# Check if GPU is available
if ! command -v nvidia-smi &>/dev/null; then
  echo "⚠️  nvidia-smi not found. Make sure NVIDIA drivers are installed."
  exit 1
fi

# Check if Docker is installed
if ! command -v docker &>/dev/null; then
  echo "❌ Docker not found. Please install Docker first."
  exit 1
fi

# Check if docker-compose is available
if ! command -v docker-compose &>/dev/null && ! docker compose version &>/dev/null; then
  echo "❌ docker-compose not found. Please install docker-compose."
  exit 1
fi

# Check NVIDIA Container Runtime
if ! docker info | grep -q nvidia; then
  echo "⚠️  NVIDIA Container Runtime not detected. Installing..."

  # Add NVIDIA package repositories
  distribution=$(
    . /etc/os-release
    echo $ID$VERSION_ID
  )
  curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | sudo apt-key add -
  curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | sudo tee /etc/apt-sources.list.d/nvidia-docker.list

  # Install nvidia-container-runtime
  sudo nala update
  sudo nala install -y nvidia-container-runtime

  # Restart Docker
  sudo systemctl restart docker

  echo "✅ NVIDIA Container Runtime installed. You may need to restart your terminal."
fi

# Create project structure
echo "📁 Creating project structure..."
mkdir -p projects
mkdir -p mcp-server

# Start Ollama service
echo "🚀 Starting Ollama service..."
docker-compose up -d ollama

# Wait for Ollama to be ready
echo "⏳ Waiting for Ollama to start..."
while ! curl -s http://localhost:11434/api/tags >/dev/null; do
  sleep 2
done

echo "📥 Downloading optimized models for 16GB VRAM..."
# Primary coding model - excellent balance of size and capability
docker exec agent-ollama ollama pull qwen3:14b

# Alternative: MoE coding model with good efficiency
docker exec agent-ollama ollama pull qwen3-coder:30b-a3b

# Smaller model for simpler tasks or if you need more VRAM headroom
docker exec agent-ollama ollama pull qwen3:8b

# Optional: Higher quality version if you want maximum coding quality
# docker exec agent-ollama ollama pull qwen3:14b:q6_k

# Test the setup
echo "🧪 Testing Ollama API..."
curl -s http://localhost:11434/api/generate -d '{
  "model": "qwen3:14b",
  "prompt": "Write a hello world function in Go with error handling",
  "stream": false
}' | jq -r '.response' | head -15

echo "✅ Setup complete!"
echo ""
echo "Next steps:"
echo "1. Test the models: curl http://localhost:11434/api/tags"
echo "2. Build the MCP server when ready"
echo "3. Create your first project with CLAUDE.md and AGENTS.md files"
echo ""
echo "Ollama is running on: http://localhost:11434"
echo "Models installed: qwen3:14b, qwen3-coder:30b-a3b, qwen3:8b"
echo ""
echo "VRAM usage (estimated):"
echo "  qwen3:14b - ~9GB (primary recommendation)"
echo "  qwen3-coder:30b-a3b - ~9GB (MoE alternative)"
echo "  qwen3:8b - ~5GB (lighter option)"
