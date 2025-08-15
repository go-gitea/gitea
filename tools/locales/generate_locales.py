#!/usr/bin/env python3
import configparser
from datetime import datetime
import os
import glob

LOCALE_DIR = "options/locale"
BASE_LANG = "locale_en-US.ini"
LOG_FILE = "log/generate_locales.log"


def read_locale_file_with_default(path):
    """Reading .ini with dummy section for lines before first section"""
    with open(path, encoding="utf-8") as f:
        content = f.read()
    content = "[__DEFAULT__]\n" + content
    parser = configparser.ConfigParser(strict=False, interpolation=None)
    parser.optionxform = str
    parser.read_string(content)
    return parser


def log_message(msg):
    print(msg)
    with open(LOG_FILE, "a", encoding="utf-8") as f:
        f.write(f"{datetime.now().isoformat()} {msg}\n")


def write_locale_file(path, default_keys, parser_lang):
    """We write the file by inserting the keys of the default section at the beginning without a section"""
    lines = []
    # add default section keys
    for key, val in default_keys.items():
        lines.append(f"{key} = {val}")

    # add the remaining sections
    for section in parser_lang.sections():
        if section == "__DEFAULT__":
            continue
        lines.append(f"\n[{section}]")
        for key, val in parser_lang[section].items():
            lines.append(f"{key} = {val}")

    with open(path, "w", encoding="utf-8") as f:
        f.write("\n".join(lines) + "\n")


def main():
    if not os.path.exists("log"):
        os.makedirs("log")

    if os.path.exists(LOG_FILE):
        os.remove(LOG_FILE)  # clearing the old log

    base_path = os.path.join(LOCALE_DIR, BASE_LANG)
    if not os.path.exists(base_path):
        log_message(f"[ERROR] Base locale file not found: {base_path}")
        return

    parser_base = read_locale_file_with_default(base_path)

    # default section keys
    default_keys = dict(parser_base["__DEFAULT__"])

    # all other keys with sections
    base_keys_with_sections = {}
    for section in parser_base.sections():
        if section == "__DEFAULT__":
            continue
        for key, val in parser_base[section].items():
            base_keys_with_sections[(section, key)] = val

    for path in glob.glob(os.path.join(LOCALE_DIR, "locale_*.ini")):
        if os.path.basename(path) == BASE_LANG:
            continue

        parser_lang = read_locale_file_with_default(path)
        modified = False

        # default section
        for key, val in default_keys.items():
            if key not in parser_lang["__DEFAULT__"]:
                parser_lang["__DEFAULT__"][key] = val
                log_message(f"[ADD KEY DEFAULT] {os.path.basename(path)} -> {key} = {val}")
                modified = True

        # other sections
        for (section, key), val in base_keys_with_sections.items():
            if not parser_lang.has_section(section):
                parser_lang.add_section(section)
                log_message(f"[ADD SECTION] {os.path.basename(path)} -> [{section}]")
                modified = True
            if not parser_lang.has_option(section, key):
                parser_lang.set(section, key, val)
                log_message(f"[ADD KEY] {os.path.basename(path)} -> [{section}] {key} = {val}")
                modified = True

        if modified:
            write_locale_file(path, parser_lang["__DEFAULT__"], parser_lang)
            log_message(f"[NEED UPDATE] {os.path.basename(path)}")


if __name__ == "__main__":
    main()
