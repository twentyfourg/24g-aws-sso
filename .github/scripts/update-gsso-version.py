#!/usr/bin/env python3
"""Update the gsso version between semantic-release anchors."""

import re
import sys
from pathlib import Path


VERSION_FILE = Path(__file__).parents[2] / "gsso" / "python" / "gsso"
VERSION_PATTERN = re.compile(
    r'(?m)(^# semantic-release-version-start\nVERSION = ")[^"]+'
    r'("\n# semantic-release-version-end$)'
)


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit(f"usage: {Path(sys.argv[0]).name} <version>")

    content = VERSION_FILE.read_text()
    updated, replacements = VERSION_PATTERN.subn(
        rf"\g<1>{sys.argv[1]}\g<2>",
        content,
    )
    if replacements != 1:
        raise SystemExit(
            f"expected one semantic-release version block, found {replacements}"
        )

    VERSION_FILE.write_text(updated)


if __name__ == "__main__":
    main()
