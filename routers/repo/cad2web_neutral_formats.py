#!/usr/bin/env python
# coding: utf-8

r"""Conversion procedure of neutral CAD format to web view"""

from __future__ import print_function, absolute_import

from os import remove, mkdir
from os.path import basename, isdir

from aocxchange.step import StepImporter
from aocxchange.iges import IgesImporter
from aocxchange.brep import BrepImporter
from aocxchange.stl import StlImporter

from aocutils.analyze.bounds import BoundingBox

from cad2web_manage_files import _conversion_filename, _descriptor_filename, \
    _write_descriptor
from cad2web_convert_shape import _convert_shape


def convert_step_file(step_filename, target_folder, remove_original=True):
    r"""Convert a STEP file (.step, .stp) for web display

    Parameters
    ----------
    step_filename : str
        Full path to the STEP file
    target_folder : str
        Full path to the target folder for the conversion
    remove_original : bool
        Should the input file be deleted after conversion?
        It should be deleted on a web platform to save disk space, but, for
        testing, it might be useful not to delete it.

    Returns
    -------
    Nothing, it is a procedure

    """
    if not isdir(target_folder):
        mkdir(target_folder)
    converted_basenames = []
    importer = StepImporter(step_filename)
    shapes = importer.shapes
    max_dim = BoundingBox(importer.compound).max_dimension
    for i, shape in enumerate(shapes):
        converted_filename = _conversion_filename(step_filename,
                                                  target_folder,
                                                  i)
        _convert_shape(shape, converted_filename)
        converted_basenames.append(basename(converted_filename))

    _write_descriptor(max_dim,
                      converted_basenames,
                      _descriptor_filename(target_folder,
                                           basename(step_filename)))
    if remove_original is True:
        remove(step_filename)


def convert_iges_file(iges_filename, target_folder, remove_original=True):
    r"""Convert an IGES file (.iges, .igs) for web display

    Parameters
    ----------
    iges_filename : str
        Full path to IGES file
    target_folder : str
        Full path to the target folder for the conversion
    remove_original : bool
        Should the input file be deleted after conversion?
        It should be deleted on a web platform to save disk space, but, for
        testing, it might be useful not to delete it.

    Returns
    -------
    Nothing, it is a procedure

    """
    if not isdir(target_folder):
        mkdir(target_folder)
    converted_basenames = []
    importer = IgesImporter(iges_filename)
    shapes = importer.shapes
    max_dim = BoundingBox(importer.compound).max_dimension
    for i, shape in enumerate(shapes):
        converted_filename = _conversion_filename(iges_filename,
                                                  target_folder,
                                                  i)
        _convert_shape(shape, converted_filename)
        converted_basenames.append(basename(converted_filename))

    _write_descriptor(max_dim,
                      converted_basenames,
                      _descriptor_filename(target_folder,
                                           basename(iges_filename)))
    if remove_original is True:
        remove(iges_filename)


def convert_brep_file(brep_filename, target_folder, remove_original=True):
    r"""Convert a BREP file (.brep, .brp) for web display

    Parameters
    ----------
    brep_filename : str
        Full path to the BREP file
    target_folder : str
        Full path to the target folder for the conversion
    remove_original : bool
        Should the input file be deleted after conversion?
        It should be deleted on a web platform to save disk space, but, for
        testing, it might be useful not to delete it.

    Returns
    -------
    Nothing, it is a procedure

    """
    if not isdir(target_folder):
        mkdir(target_folder)
    shape = BrepImporter(brep_filename).shape
    converted_filename = _conversion_filename(brep_filename,
                                              target_folder,
                                              0)
    converted_basenames = [basename(converted_filename)]
    _convert_shape(shape, converted_filename)

    _write_descriptor(BoundingBox(shape).max_dimension,
                      converted_basenames,
                      _descriptor_filename(target_folder,
                                           basename(brep_filename)))
    if remove_original is True:
        remove(brep_filename)


def convert_stl_file(stl_filename, target_folder, remove_original=True):
    r"""Convert a STL file (.stl) for web display

    Parameters
    ----------
    stl_filename : str
        Full path to the STL file
    target_folder : str
        Full path to the target folder for the conversion
    remove_original : bool
        Should the input file be deleted after conversion?
        It should be deleted on a web platform to save disk space, but, for
        testing, it might be useful not to delete it.

    Returns
    -------
    Nothing, it is a procedure

    """
    if not isdir(target_folder):
        mkdir(target_folder)
    importer = StlImporter(stl_filename)
    converted_filename = _conversion_filename(stl_filename,
                                              target_folder,
                                              0)
    converted_basenames = [basename(converted_filename)]
    _convert_shape(importer.shape, converted_filename)
    max_dim = BoundingBox(importer.shape).max_dimension
    _write_descriptor(max_dim,
                      converted_basenames,
                      _descriptor_filename(target_folder,
                                           basename(stl_filename)))
    if remove_original is True:
        remove(stl_filename)
