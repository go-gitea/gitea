#!/usr/bin/env python
# coding: utf-8

r"""Tests for cad2web_neutral_formats.py"""

import pytest

from shutil import rmtree

from os.path import join, dirname, isfile, basename

from cad2web_py import convert_py_file_part
from cad2web_manage_files import _descriptor_filename


def test_convert_py_file_part():
    r"""Test the conversion of a PY file that contains
    the definition of a part"""
    relpath = "tests/in/py/sample_projects/test_project/py_scripts/plate_with_holes.py"
    path_py_file_part = join(dirname(__file__), relpath)
    target_folder = join(dirname(__file__), "tests/out/plate_with_holes_py")
    convert_py_file_part(path_py_file_part, target_folder, remove_original=False)
    assert isfile(_descriptor_filename(target_folder,
                                       basename(path_py_file_part)))
    rmtree(target_folder, ignore_errors=True)
