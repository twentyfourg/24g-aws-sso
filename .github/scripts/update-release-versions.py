#!/usr/bin/env python3
"""Update versions between semantic-release anchors."""

import re
import sys
from datetime import date
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]

GSSO_FILE = ROOT / "gsso" / "python" / "gsso"
GSSO_PATTERN = re.compile(
    r'(?m)(^# semantic-release-version-start\nVERSION = ")[^"]+'
    r'("\n# semantic-release-version-end$)'
)

SETUP_FILES = [
    ROOT / "setup-instructions" / "24GAWSMCPSetup.md",
    ROOT / "setup-instructions" / "24GAWSSOSetup.md",
]
SETUP_PATTERN = re.compile(
    r"(?m)(^<!-- semantic-release-version-start -->\nVersion: )\S+"
    r"(\nUpdated: )\S+"
    r"(\n<!-- semantic-release-version-end -->$)"
)


def replace_once(path: Path, pattern: re.Pattern[str], replacement: str) -> None:
    content = path.read_text()
    updated, replacements = pattern.subn(replacement, content)
    if replacements != 1:
        raise SystemExit(
            f"{path}: expected one semantic-release version block, found {replacements}"
        )
    path.write_text(updated)


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit(f"usage: {Path(sys.argv[0]).name} <version>")

    version = sys.argv[1]
    today = date.today().isoformat()

    replace_once(GSSO_FILE, GSSO_PATTERN, rf"\g<1>{version}\g<2>")
    for path in SETUP_FILES:
        replace_once(
            path,
            SETUP_PATTERN,
            rf"\g<1>{version}\g<2>{today}\g<3>",
        )


if __name__ == "__main__":
    main()
