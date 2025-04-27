# AIDEA Activity Tracker

A collection of Go tools for tracking activity descriptions using vector embeddings and RAG (Retrieval Augmented Generation).

## Overview

This project provides a set of Go programs that use Weaviate (a vector database) to:

1. Create collections for storing activity descriptions
2. Import activity data from JSON files
3. Perform similarity searches to find relevant activities
4. Generate responses about activities using RAG techniques

## Getting Started

### Prerequisites

- Go 1.24.2 or higher
- Weaviate running locally on port 8080
- Ollama running locally on port 11434 with the following models:
  - all-minilm (for embeddings)
  - gemma3 (for generation)

### Installation

1. Clone this repository
2. Install dependencies: `go mod download`

### Usage

See [CLAUDE.md](CLAUDE.md) for detailed usage instructions and documentation.

### Basic Flow

1. Check if Weaviate is ready:
   ```
   go run check_readiness.go
   ```

2. Create a collection:
   ```
   go run AIDEA_create_collection.go
   ```

3. Import activity data:
   ```
   go run AIDEA_import_data.go
   ```

4. Search for similar activities:
   ```
   go run AIDEA_near_text.go
   ```

5. Generate responses about activities:
   ```
   go run AIDEA_rag.go
   ```

## License

This project is provided as-is with no explicit license.

## Acknowledgments

- Based on examples from the [Weaviate quickstart guide](https://weaviate.io/developers/weaviate/quickstart/local)