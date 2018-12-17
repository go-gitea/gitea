#!/usr/bin/env python
# coding: utf-8

r"""cad2web benchmark"""

from shutil import rmtree
import time
import uuid
from os.path import join, dirname

from cad2web_freecad import convert_freecad_file
from cad2web_neutral_formats import convert_step_file, convert_iges_file, \
    convert_brep_file, convert_stl_file
from cad2web_py import convert_py_file_part


def convert_timed(path, func):
    r"""Test name file visibility tuples from known FCSTD file"""
    path = join(dirname(__file__), path)
    target_folder = join(dirname(__file__), "tests/out/%s" % str(uuid.uuid4()))
    t0 = time.time()
    func(path, target_folder, remove_original=False)
    t1 = time.time()
    print("Conversion took %.3f s for %s" % ((t1 - t0), path))
    rmtree(target_folder, ignore_errors=True)


def main():

    print("**** BREP ****")
    convert_timed("tests/in/brep/cylinder_head.brep", convert_brep_file)
    convert_timed("tests/in/brep/Motor-c.brep", convert_brep_file)
    print("")

    print("**** FREECAD ****")
    convert_timed("tests/in/freecad/cyl_on_cube.FCStd", convert_freecad_file)
    # convert_timed("tests/in/freecad/wind_tunnel_complete_bellmouth.FCStd",
    #               convert_freecad_file)
    print("")

    print("**** IGES ****")
    convert_timed("tests/in/iges/aube_pleine.iges", convert_iges_file)
    convert_timed("tests/in/iges/box.igs", convert_iges_file)
    convert_timed("tests/in/iges/bracket.igs", convert_iges_file)
    print("")
    print("**** PY ****")
    convert_timed("tests/in/py/sample_projects/test_project/py_scripts/plate_with_holes.py",
                  convert_py_file_part)
    print("")

    print("**** STEP ****")
    convert_timed("tests/in/step/11752.stp", convert_step_file)
    convert_timed("tests/in/step/as1_pe_203.stp", convert_step_file)
    convert_timed("tests/in/step/bottle.stp", convert_step_file)
    print("")

    print("**** STL ****")
    convert_timed("tests/in/stl/box_ascii.stl", convert_stl_file)
    convert_timed("tests/in/stl/box_binary.stl", convert_stl_file)
    print("")


if __name__ == "__main__":
    main()
