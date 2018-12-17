#!/usr/bin/env python
# coding: utf-8

r"""Tests for cad2web_neutral_formats.py"""

import pytest

from shutil import rmtree

from os.path import join, dirname, isfile, basename

from cad2web_neutral_formats import convert_step_file, \
    convert_iges_file, convert_brep_file, convert_stl_file
from cad2web_manage_files import _descriptor_filename


def test_convert_step_file():
    r"""Test the conversion of a STEP file"""
    path_step = join(dirname(__file__), "tests/in/step/bottle.stp")
    target_folder = join(dirname(__file__), "tests/out/bottle")
    convert_step_file(path_step, target_folder, remove_original=False)
    assert isfile(_descriptor_filename(target_folder, basename(path_step)))
    rmtree(target_folder, ignore_errors=True)


def test_convert_iges_file():
    r"""Test the conversion of a IGES file"""
    path_iges = join(dirname(__file__), "tests/in/iges/aube_pleine.iges")
    target_folder = join(dirname(__file__), "tests/out/aube_pleine")
    convert_iges_file(path_iges, target_folder, remove_original=False)
    assert isfile(_descriptor_filename(target_folder, basename(path_iges)))
    rmtree(target_folder, ignore_errors=True)


def test_convert_brep_file():
    r"""Test the conversion of a BREP file"""
    path_brep = join(dirname(__file__), "tests/in/brep/cylinder_head.brep")
    target_folder = join(dirname(__file__), "tests/out/cylinder_head")
    convert_brep_file(path_brep, target_folder, remove_original=False)
    assert isfile(_descriptor_filename(target_folder, basename(path_brep)))
    rmtree(target_folder, ignore_errors=True)


def test_convert_stl_file_ascii():
    r"""Test the conversion of an ASCII STL file"""
    path_stl = join(dirname(__file__), "tests/in/stl/box_ascii.stl")
    target_folder = join(dirname(__file__), "tests/out/box_ascii")
    convert_stl_file(path_stl, target_folder, remove_original=False)
    assert isfile(_descriptor_filename(target_folder, basename(path_stl)))
    rmtree(target_folder, ignore_errors=True)


def test_convert_stl_file_binary():
    r"""Test the conversion of a binary STL file"""
    path_stl = join(dirname(__file__), "tests/in/stl/box_binary.stl")
    target_folder = join(dirname(__file__), "tests/out/box_binary")
    convert_stl_file(path_stl, target_folder, remove_original=False)
    assert isfile(_descriptor_filename(target_folder, basename(path_stl)))
    rmtree(target_folder, ignore_errors=True)
