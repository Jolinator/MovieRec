#!/bin/sh
set -eu

download_model() {
    target="$1"
    url="$2"
    expected_sha256="$3"
    temporary="${target}.download"

    if [ -f "$target" ]; then
        actual_sha256="$(sha256sum "$target" | cut -d ' ' -f 1)"
        if [ "$actual_sha256" = "$expected_sha256" ]; then
            echo "Model already present and verified: $target"
            return
        fi
        echo "Checksum mismatch for existing model: $target" >&2
        rm -f "$target"
    fi

    echo "Downloading $target"
    rm -f "$temporary"
    curl --fail --location --retry 3 --retry-delay 5 --output "$temporary" "$url"
    actual_sha256="$(sha256sum "$temporary" | cut -d ' ' -f 1)"
    if [ "$actual_sha256" != "$expected_sha256" ]; then
        echo "Checksum mismatch after downloading $target" >&2
        rm -f "$temporary"
        exit 1
    fi
    mv "$temporary" "$target"
}

download_model \
    /models/embedding.gguf \
    "$EMBEDDING_MODEL_URL" \
    "$EMBEDDING_MODEL_SHA256"

download_model \
    /models/qwen.gguf \
    "$GENERATION_MODEL_URL" \
    "$GENERATION_MODEL_SHA256"

echo "All inference models are present and verified."
