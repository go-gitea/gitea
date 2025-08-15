#!/usr/bin/env python3
import configparser
from datetime import datetime
import os
import glob

LOCALE_DIR = "options/locale"
BASE_LANG = "locale_en-US.ini"  # always English
LOG_FILE = "log/search_not_exist_EN_us_keys_locales.log"

def read_locale_file_with_default(path):
    """Reading .ini with adding dummy section for lines before first section"""
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

def main():
    print("start...")
    if os.path.exists(LOG_FILE):
        os.remove(LOG_FILE)  # clearing the old log

    base_path = os.path.join(LOCALE_DIR, BASE_LANG)
    if not os.path.exists(base_path):
        log_message(f"[ERROR] Base locale file not found: {base_path}")
        return

    # Reading English keys
    parser_base = read_locale_file_with_default(base_path)
    base_keys = set()
    for section in parser_base.sections():
        for key in parser_base[section]:
            base_keys.add((section, key))

    # Пwe're going through all the other languages
    for path in glob.glob(os.path.join(LOCALE_DIR, "locale_*.ini")):
        if os.path.basename(path) == BASE_LANG:
            continue
        parser_lang = read_locale_file_with_default(path)

        for section in parser_lang.sections():
            for key in parser_lang[section]:
                if (section, key) not in base_keys:
                    log_message(f"[EXTRA KEY] {os.path.basename(path)} -> [{section}] {key} (does not exist in {BASE_LANG})")
                    print(f"[EXTRA KEY] {os.path.basename(path)} -> [{section}] {key}")
    print("stop...")

if __name__ == "__main__":
    main()
