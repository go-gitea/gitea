#!/usr/bin/env python3
from collections import defaultdict
from subprocess import check_output

lines = check_output(["go", "run", "./cmd/chroma/main.go", "--list"]).decode('utf-8').splitlines()
lines = [line.strip() for line in lines if line.startswith("  ") and not line.startswith("   ")]
lines = sorted(lines, key=lambda l: l.lower())

table = defaultdict(list)

for line in lines:
    table[line[0].upper()].append(line)

for key, value in table.items():
    print("{} | {}".format(key, ', '.join(value)))
