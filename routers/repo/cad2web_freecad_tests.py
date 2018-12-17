#!/usr/bin/env python
# coding: utf-8

r"""Tests for cad2web_freecad.py"""

import pytest

from shutil import rmtree

from os.path import join, dirname

from cad2web_freecad import name_file_visibility_from_unzipping_folder, \
    extract_fcstd


def test_name_file_visibility_from_fcstd():
    r"""Test name file visibility tuples from known FCSTD file"""
    path_fcstd = join(dirname(__file__), "tests/in/freecad/cyl_on_cube.FCStd")
    target_folder = join(dirname(__file__), "tests/out/cyl_on_cube")
    extract_fcstd(path_fcstd, target_folder)
    nfv_tuples = name_file_visibility_from_unzipping_folder(target_folder)
    assert nfv_tuples == [('Box', 'PartShape.brp', True),
                          ('Cylinder', 'PartShape1.brp', True)]
    rmtree(target_folder, ignore_errors=True)
