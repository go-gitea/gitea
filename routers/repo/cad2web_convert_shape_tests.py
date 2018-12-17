#!/usr/bin/env python
# coding: utf-8

r"""Tests for cad2web_convert_shape.py"""

import pytest

from time import time

from os import remove
from os.path import join, dirname, isfile

from OCC.Core.BRepPrimAPI import BRepPrimAPI_MakeBox

from cad2web_convert_shape import _convert_shape


def test_convert_shape():
    my_box = BRepPrimAPI_MakeBox(10., 20., 30.).Shape()
    target = join(dirname(__file__), "tests/out/box.json")
    _convert_shape(my_box, target)
    assert isfile(target)
    remove(target)


def test_convert_shape_cache():
    my_box = BRepPrimAPI_MakeBox(10., 20., 30.).Shape()
    target = join(dirname(__file__), "tests/out/box.json")
    assert not isfile(target)
    t0 = time()
    _convert_shape(my_box, target)
    t1 = time()
    assert isfile(target)
    t2 = time()
    _convert_shape(my_box, target)  # this call should use cached target
    t3 = time()
    assert isfile(target)
    assert t3 - t2 < t1 - t0
    remove(target)

