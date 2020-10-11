#!/usr/bin/env python
# coding: utf-8

r"""Conversion procedure of Gitea for CAD.

Called from Go code. The parameters are passed as command line arguments
in the Go code.

"""

from __future__ import print_function, absolute_import

import imp
import logging
from os import remove, getcwd, chdir, system, mkdir
from os.path import splitext, basename, isdir
import sys


from aocutils.analyze.bounds import BoundingBox

from osvcad.nodes import Part

from cad2web_manage_files import _conversion_filename, _descriptor_filename, \
    _write_descriptor
from cad2web_convert_shape import _convert_shape


logger = logging.getLogger(__name__)


def convert_py_file_part(py_filename, target_folder, remove_original=True):
    r"""Convert an OsvCad Python file that contains a part for web display

    The Python file contains the definition of a part.

    Parameters
    ----------
    py_filename : str
        Full path to the Python file
    target_folder : str
        Full path to the target folder for the conversion
    remove_original : bool
        Should the input file be deleted after conversion?
        It should be deleted on a web platform to save disk space, but, for
        testing, it might be useful not to delete it.

    Returns
    -------
    Nothing, it is a procedure

    Raises
    ------
    ValueError if not a part definition

    """
    if not isdir(target_folder):
        mkdir(target_folder)
    part = Part.from_py_script(py_filename)
    shape = part.node_shape.shape
    converted_filename = _conversion_filename(py_filename,
                                              target_folder,
                                              0)
    converted_basenames = [basename(converted_filename)]
    _convert_shape(shape, converted_filename)

    _write_descriptor(BoundingBox(shape).max_dimension,
                      converted_basenames,
                      _descriptor_filename(target_folder,
                                           basename(py_filename)))
    if remove_original is True:
        remove(py_filename)


def convert_py_file_assembly(py_filename,
                             target_folder,
                             clone_url,
                             branch,
                             remove_original=True):
    r"""

    **** WORK IN PROGRESS ****

    Parameters
    ----------
    py_filename
    target_folder
    clone_url
    branch

    Returns
    -------

    """
    if not isdir(target_folder):
        mkdir(target_folder)

    logger.info("Dealing with a Python file that is supposed to "
                "define an assembly")
    # -1- change working dir to converted_files
    working_dir_initial = getcwd()
    chdir(target_folder)
    # 0 - Git clone
    logger.info("Git cloning %s into %s" % (clone_url, target_folder))

    # from subprocess import call
    # call(["cd", target_folder, "&&", "git", "clone", clone_url])
    system("cd %s && git clone %s" % (target_folder, clone_url))

    project = clone_url.split("/")[-1]

    # 1 - Git checkout the right branch/commit
    logger.info("Git checkout %s of %s" % (branch, project))
    system("cd %s/%s && git checkout %s" % (target_folder,
                                               project,
                                               branch))

    # 2 - Alter sys.path
    logger.info("Git checkout %s of %s" % (branch, project))
    sys_path_initial = sys.path
    path_extra = "%s/%s" % (target_folder, project)
    logger.info("Appending sys.path with %s" % path_extra)
    sys.path.append(path_extra)

    # Useless : adding converted_files to sys.path
    # logger.info("Appending sys.path with %s" % dirname(py_filename))
    # sys.path.append(dirname(py_filename))

    # 3 - Run osvcad functions
    converted_basenames = []
    # TODO : THE PROBLEM IS THAT WE ARE IMPORTING THE FILE OUTSIDE OF THE
    # CONTEXT OF ITS PROJECT
    module_ = imp.load_source(splitext(basename(py_filename))[0],
                              py_filename)
    assembly = getattr(module_, "assembly")

    for i, node in enumerate(assembly.nodes()):
        shape = node.node_shape.shape
        converted_filename = _conversion_filename(py_filename,
                                                  target_folder,
                                                  i)
        converted_basenames.append(basename(converted_filename))
        _convert_shape(shape, converted_filename)
    # TODO : max_dim
    _write_descriptor(1000,
                      converted_basenames,
                      _descriptor_filename(target_folder,
                                           basename(py_filename)))
    remove(py_filename)

    # 4 - Put sys.path back to where it was
    sys.path = sys_path_initial
    # 5 - Set back the working dir
    chdir(working_dir_initial)
    # 6 - Cleanup
    if remove_original is True:
        pass

    # TODO : remove folder created by git clone or the next clone will fail


def convert_py_file(py_filename,
                    target_folder,
                    clone_url,
                    branch,
                    remove_original=True):
    r"""Convert an OsvCad Python file for web display

    The Python file can contain the definition of a part or of an assembly

    Parameters
    ----------
    py_filename : str
        Full path to the Python file
    target_folder : str
        Full path to the target folder for the conversion

    Returns
    -------
    Nothing, it is a procedure

    """
    try:
        convert_py_file_part(py_filename,
                             target_folder,
                             remove_original=remove_original)
    except (ValueError, FileNotFoundError, ImportError):
        # probably no part attribute in module
        msg = "No part attribute in module"
        logger.warning(msg)
        try:
            convert_py_file_assembly(py_filename,
                                     target_folder,
                                     clone_url,
                                     branch,
                                     remove_original=remove_original)

        except AttributeError:
            msg = "No part nor assembly attribute in module"
            logger.error(msg)
