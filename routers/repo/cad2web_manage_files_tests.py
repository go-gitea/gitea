#!/usr/bin/env python
# coding: utf-8

r"""Tests for cad2web_manage_files.py

Requires an internet connection to test the _download_file() function

"""

from os import remove
from os.path import isfile, join, dirname

import pytest

import requests

from cad2web_manage_files import _download_file, sha1


def test_download_file():
    r"""Test the download of a file that exists"""
    TEST_FILE_URL = "https://raw.githubusercontent.com/osv-team/pyosv/master/" \
                    "article.rst"
    TEST_FILE_NAME = join(dirname(__file__), "tmp_test.rst")
    _download_file(TEST_FILE_URL, TEST_FILE_NAME)
    assert isfile(TEST_FILE_NAME)
    remove(TEST_FILE_NAME)


def test_download_file_wrong_url():
    r"""Test the download of a file that does not exist"""
    TEST_FILE_URL = "https://raw.githubusercontent.com/osv-team/pyosv/" \
                    "master/a_r_t_i_c_l_e.rst"
    TEST_FILE_NAME = join(dirname(__file__), "tmp_test.rst")

    with pytest.raises(requests.exceptions.HTTPError):
        _download_file(TEST_FILE_URL, TEST_FILE_NAME)

    # Make sure no file is create
    assert not isfile(TEST_FILE_NAME)


def test_download_file_stupid_target():
    TEST_FILE_URL = "https://raw.githubusercontent.com/osv-team/pyosv/master/" \
                    "article.rst"
    TEST_FILE_NAME = join("/unknown/", "tmp_test.rst")
    with pytest.raises(FileNotFoundError):
        _download_file(TEST_FILE_URL, TEST_FILE_NAME)


def test_sha1():
    r"""Test the sha1() function on a file which content
    is supposed to be stable"""
    assert sha1(join(dirname(__file__), "topic.go")) == \
        "d42689f18e579fd70f155f8745bccf9487f1c5fe"


def test_sha1_non_existent_file():
    r"""Test sha1 for a file that does not exist"""
    with pytest.raises(FileNotFoundError):
        sha1(join(dirname(__file__), "unknown_file.go"))
