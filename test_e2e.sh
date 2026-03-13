#!/bin/bash
# End-to-end test script for PiBot
# This script starts the server, runs tests, and cleans up

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== PiBot End-to-End Tests ===${NC}"
echo ""

# Build the project
echo -e "${YELLOW}Building PiBot...${NC}"
go build -o pibot ./cmd/pibot
echo -e "${GREEN}✓ Build successful${NC}"
echo ""

# Run unit tests
echo -e "${YELLOW}Running unit tests...${NC}"
go test -v ./internal/... 2>&1 | while IFS= read -r line; do
    if [[ "$line" == *"PASS"* ]]; then
        echo -e "${GREEN}$line${NC}"
    elif [[ "$line" == *"FAIL"* ]]; then
        echo -e "${RED}$line${NC}"
    elif [[ "$line" == *"==="* ]]; then
        echo -e "${YELLOW}$line${NC}"
    else
        echo "$line"
    fi
done

# Check if all tests passed
go test ./internal/... > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ All unit tests passed${NC}"
else
    echo -e "${RED}✗ Some unit tests failed${NC}"
    exit 1
fi
echo ""

# Start server in background for integration tests
echo -e "${YELLOW}Starting PiBot server for integration tests...${NC}"
./pibot &
SERVER_PID=$!
sleep 2

# Check if server started
if ! kill -0 $SERVER_PID 2>/dev/null; then
    echo -e "${RED}✗ Failed to start server${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Server started (PID: $SERVER_PID)${NC}"
echo ""

# Function to cleanup
cleanup() {
    echo ""
    echo -e "${YELLOW}Cleaning up...${NC}"
    if kill -0 $SERVER_PID 2>/dev/null; then
        kill $SERVER_PID 2>/dev/null
        wait $SERVER_PID 2>/dev/null || true
    fi
    echo -e "${GREEN}✓ Server stopped${NC}"
}
trap cleanup EXIT

# Run integration tests
echo -e "${YELLOW}Running integration tests...${NC}"
FAILED=0

# Test 1: Config endpoint
echo -n "  Testing GET /api/config... "
RESPONSE=$(curl -s http://localhost:8080/api/config)
if echo "$RESPONSE" | grep -q "default_provider"; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC}"
    FAILED=1
fi

# Test 2: Providers endpoint
echo -n "  Testing GET /api/providers... "
RESPONSE=$(curl -s http://localhost:8080/api/providers)
if echo "$RESPONSE" | grep -q "providers"; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC}"
    FAILED=1
fi

# Test 3: Safe command execution
echo -n "  Testing POST /api/exec (safe command)... "
RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"command":"echo test123"}' \
    http://localhost:8080/api/exec)
if echo "$RESPONSE" | grep -q "test123"; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC}"
    FAILED=1
fi

# Test 4: Dangerous command should be pending
echo -n "  Testing POST /api/exec (dangerous command)... "
RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"command":"rm -rf /tmp/test"}' \
    http://localhost:8080/api/exec)
if echo "$RESPONSE" | grep -q '"pending":true'; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC}"
    FAILED=1
fi

# Test 5: Blocked command should be rejected
echo -n "  Testing POST /api/exec (blocked command)... "
RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"command":"dd if=/dev/zero of=/dev/sda"}' \
    http://localhost:8080/api/exec)
if echo "$RESPONSE" | grep -q "blocked"; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC}"
    FAILED=1
fi

# Test 6: File listing
echo -n "  Testing GET /api/files... "
RESPONSE=$(curl -s http://localhost:8080/api/files)
if echo "$RESPONSE" | grep -q "base_directory"; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC}"
    FAILED=1
fi

# Test 7: File write
echo -n "  Testing POST /api/files/test.txt... "
RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"content":"e2e test content"}' \
    http://localhost:8080/api/files/e2e-test.txt)
if echo "$RESPONSE" | grep -q '"status":"written"'; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC}"
    FAILED=1
fi

# Test 8: File read
echo -n "  Testing GET /api/files/e2e-test.txt... "
RESPONSE=$(curl -s http://localhost:8080/api/files/e2e-test.txt)
if echo "$RESPONSE" | grep -q "e2e test content"; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC}"
    FAILED=1
fi

# Test 9: File delete
echo -n "  Testing DELETE /api/files/e2e-test.txt... "
RESPONSE=$(curl -s -X DELETE http://localhost:8080/api/files/e2e-test.txt)
if echo "$RESPONSE" | grep -q '"status":"deleted"'; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC}"
    FAILED=1
fi

# Test 10: Static files (HTML)
echo -n "  Testing GET / (static HTML)... "
RESPONSE=$(curl -s http://localhost:8080/)
if echo "$RESPONSE" | grep -q "PiBot"; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC}"
    FAILED=1
fi

# Test 11: Static files (CSS)
echo -n "  Testing GET /css/style.css... "
RESPONSE=$(curl -s http://localhost:8080/css/style.css)
if echo "$RESPONSE" | grep -q ":root"; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC}"
    FAILED=1
fi

# Test 12: Static files (JS)
echo -n "  Testing GET /js/app.js... "
RESPONSE=$(curl -s http://localhost:8080/js/app.js)
if echo "$RESPONSE" | grep -q "class PiBot"; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC}"
    FAILED=1
fi

# Test 13: Settings page
echo -n "  Testing GET /settings.html... "
RESPONSE=$(curl -s http://localhost:8080/settings.html)
if echo "$RESPONSE" | grep -q "Settings"; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC}"
    FAILED=1
fi

# Test 14: Config update
echo -n "  Testing POST /api/config... "
RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"default_provider":"ollama"}' \
    http://localhost:8080/api/config)
if echo "$RESPONSE" | grep -q '"status":"updated"'; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC}"
    FAILED=1
fi

# Test 15: List pending commands
echo -n "  Testing GET /api/exec/pending... "
RESPONSE=$(curl -s http://localhost:8080/api/exec/pending)
if echo "$RESPONSE" | grep -q "pending"; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${RED}FAIL${NC}"
    FAILED=1
fi

echo ""

# Summary
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}=== All tests passed! ===${NC}"
    exit 0
else
    echo -e "${RED}=== Some tests failed ===${NC}"
    exit 1
fi
