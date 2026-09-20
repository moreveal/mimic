"""Check explicit parallel/serial policy for internal/browser root tests."""

from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[2]
TEST_DIR = ROOT / "internal" / "browser"
ROOT_TEST = re.compile(r"^func (Test\w+)\(t \*testing\.T\)\s*\{", re.MULTILINE)
MARKER = re.compile(r"\b(parallelBrowserTest|serialBrowserTest)\(t\)")
PARALLEL_FORBIDDEN = (
    "t.Setenv(",
    "os.Setenv(",
    "os.Unsetenv(",
    "runtime.GOMAXPROCS(",
    "profileProcessMemory",
    "context.WithTimeout(",
)


def function_body(source, opening):
    depth = 0
    index = opening
    state = "code"
    while index < len(source):
        char = source[index]
        following = source[index + 1] if index + 1 < len(source) else ""
        if state == "line-comment":
            if char == "\n":
                state = "code"
        elif state == "block-comment":
            if char == "*" and following == "/":
                state = "code"
                index += 1
        elif state == "raw-string":
            if char == "`":
                state = "code"
        elif state in ("string", "rune"):
            if char == "\\":
                index += 1
            elif (state == "string" and char == '"') or (state == "rune" and char == "'"):
                state = "code"
        elif char == "/" and following == "/":
            state = "line-comment"
            index += 1
        elif char == "/" and following == "*":
            state = "block-comment"
            index += 1
        elif char == "`":
            state = "raw-string"
        elif char == '"':
            state = "string"
        elif char == "'":
            state = "rune"
        elif char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0:
                return source[opening + 1:index]
        index += 1
    raise ValueError("unterminated function body")


def violations(test_dir=TEST_DIR):
    errors = []
    for path in sorted(test_dir.glob("*_test.go")):
        source = path.read_text(encoding="utf-8")
        for match in ROOT_TEST.finditer(source):
            name = match.group(1)
            body = function_body(source, match.end() - 1)
            markers = MARKER.findall(body)
            try:
                display_path = path.relative_to(ROOT)
            except ValueError:
                display_path = path.name
            label = f"{display_path}:{name}"
            if len(markers) != 1:
                errors.append(f"{label}: expected exactly one root policy marker")
                continue
            if markers[0] == "parallelBrowserTest":
                for token in PARALLEL_FORBIDDEN:
                    if token in body:
                        errors.append(f"{label}: parallel test uses {token}")
    return errors


def main():
    errors = violations()
    if errors:
        print("\n".join(errors), file=sys.stderr)
        return 1
    count = sum(1 for path in TEST_DIR.glob("*_test.go")
                for _ in ROOT_TEST.finditer(path.read_text(encoding="utf-8")))
    print(f"browser test policy: {count} root tests classified")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
