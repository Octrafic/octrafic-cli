#!/usr/bin/env bash

set -e

export GOTOOLCHAIN=auto

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}Starting local CI checks...${NC}\n"

if ! command -v golangci-lint &> /dev/null; then
    echo -e "${YELLOW}golangci-lint not found. Installing...${NC}"
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
    
    export PATH=$PATH:$(go env GOPATH)/bin
fi

echo -e "${YELLOW}Running golangci-lint...${NC}"
golangci-lint run
echo -e "${GREEN}✓ Linter passed${NC}\n"

echo -e "${YELLOW}Running unit tests...${NC}"
go test -v ./...
echo -e "${GREEN}✓ Tests passed${NC}\n"

echo -e "${YELLOW}Verifying build...${NC}"
GOTOOLCHAIN=auto go build -v ./cmd/octrafic
rm -f ./octrafic
echo -e "${GREEN}✓ Build verified${NC}\n"

echo -e "${GREEN}All checks passed! You are ready to commit and push.${NC}"
