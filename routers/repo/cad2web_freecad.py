#!/usr/bin/env python
# coding: utf-8

r"""Conversion procedure of a FreeCAD file."""

from __future__ import print_function, absolute_import

import logging
from os import remove
from os.path import basename, join, splitext
from shutil import rmtree
import zipfile
import xml.etree.ElementTree

from aocxchange.brep import BrepImporter

from aocutils.analyze.bounds import BoundingBox

from cad2web_manage_files import _conversion_filename, _descriptor_filename, \
    _write_descriptor
from cad2web_convert_shape import _convert_shape

logger = logging.getLogger(__name__)


def xml_root(xml_filepath):
    r"""Get the XML root element of an XML file

    Parameters
    ----------
    xml_filepath : str
        Full path to the XML file

    Returns
    -------
    XML root element

    Raises
    ------
    FileNotFoundError if xml_filepath points to a file that does not exists

    """
    return xml.etree.ElementTree.parse(xml_filepath).getroot()


def list_objects(doc_root, container="Objects"):
    r"""

    Parameters
    ----------
    doc_root : xml element
        Root element of a Document.xml file
    container : str
        Name of the XML container for the objects

    Returns
    -------
    List of XML element corresponding to the Objects

    """
    if container not in ["Objects", "ObjectData"]:
        raise ValueError("Unknown container")
    objects = []
    for objects_entry in doc_root.findall(container):
        for object_entry in objects_entry.findall('Object'):
            objects.append(object_entry)
    return objects


def name_file(doc_root):
    r"""

    Parameters
    ----------
    doc_root : xml element
        Root element of the Document.xml file

    Returns
    -------
    list of tuples
        List of (name, file) tuples from the Document.xml file

    """
    name_file_tuples = []
    lo = list_objects(doc_root, container="Objects")
    lod = list_objects(doc_root, container="ObjectData")
    for lo_ in lo:
        name = lo_.attrib['name']
        object_data_entry = filter(lambda e: e.attrib['name'] == name, lod)
        for de in object_data_entry:
            for properties in de.findall("Properties"):
                for prop in filter(lambda e: e.attrib['name'] == "Shape",
                                   properties.findall("Property")):
                    for part in prop.findall("Part"):
                        name_file_tuples.append((name, part.attrib['file']))
    return name_file_tuples


def name_visibility(guidoc_root, name):
    r"""Using GuiDocument.xml, determine the visibility of an object

    Parameters
    ----------
    guidoc_root : xml element
        XML root element of the GuiDocument.xml file
    name : str
        Object name

    Returns
    -------
    Bool
        True if visible, False otherwise

    """
    for vpd in guidoc_root.findall("ViewProviderData"):
        for vp in filter(lambda x: x.attrib['name'] == name,
                         vpd.findall("ViewProvider")):
            for ps in vp.findall("Properties"):
                for p in filter(lambda x: x.attrib['name'] == "Visibility",
                                ps.findall("Property")):
                    for b in p.findall("Bool"):
                        if b.attrib["value"] == "true":
                            return True
                        else:
                            return False


def name_file_visibility(name_file_tuples, guidoc_root):
    r"""Build a list of name, file, visibility tuples

    Parameters
    ----------
    name_file_tuples : list of tuples
        List of (name, file) tuples
    guidoc_root : xml element
        XML root element of the GuiDocument.xml file

    Returns
    -------
    list of tuples
        List of (name, file, visibility) tuples

    """
    name_file_visibility_tuples = []
    for name, file in name_file_tuples:
        visibility = name_visibility(guidoc_root, name)
        name_file_visibility_tuples.append((name, file, visibility))
    return name_file_visibility_tuples


def extract_fcstd(fcstd_filename, target_folder):
    r"""Extract a FreeCAD file to a target folder"""
    fcstd_as_zip = zipfile.ZipFile(fcstd_filename)

    for filename in fcstd_as_zip.namelist():
        fcstd_as_zip.extract(filename, target_folder)


def name_file_visibility_from_unzipping_folder(folder_unzipping):
    r"""Build a list of name, file, visibility tuples using a folder where the
    FreeCAD FCSTD file has been previously unzipped

    Parameters
    ----------
    folder_unzipping : str
        Path to the folder where the FCSTD file has been previously unzipped

    """
    docroot = xml_root(join(folder_unzipping, "Document.xml"))
    guidocroot = xml_root(join(folder_unzipping, "GuiDocument.xml"))

    name_files_tuples = name_file(docroot)
    return name_file_visibility(name_files_tuples, guidocroot)


def convert_freecad_file(freecad_filename,
                         target_folder,
                         remove_original=True):
    r"""Convert a FreeCAD file (.fcstd) for web display

    Parameters
    ----------
    freecad_filename : str
        Full path to FreeCAD file
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
    logger.info("Starting FreeCAD conversion")

    folder_unzipping = join(target_folder,
                            splitext(basename(freecad_filename))[0])

    extract_fcstd(freecad_filename, folder_unzipping)
    name_file_visibility_tuples = \
        name_file_visibility_from_unzipping_folder(folder_unzipping)

    extremas = []
    converted_basenames = []

    for i, (n, f, v) in enumerate(name_file_visibility_tuples):
        if v is True:
            brep_filename = "%s/%s" % (folder_unzipping, f)
            converted_filename = _conversion_filename(brep_filename,
                                                      target_folder,
                                                      i)
            converted_basenames.append(basename(converted_filename))
            try:
                importer = BrepImporter(brep_filename)
                extremas.append(BoundingBox(importer.shape).as_tuple)
                _convert_shape(importer.shape, converted_filename)
            except (RuntimeError, AssertionError):
                # An AssertionError is raised if one of the BREPs contained
                # in the FCSTD file contains a NULL shape
                logger.error("RuntimeError for %s" % brep_filename)

    x_min = min([extrema[0] for extrema in extremas])
    y_min = min([extrema[1] for extrema in extremas])
    z_min = min([extrema[2] for extrema in extremas])
    x_max = max([extrema[3] for extrema in extremas])
    y_max = max([extrema[4] for extrema in extremas])
    z_max = max([extrema[5] for extrema in extremas])

    max_dim = max([x_max - x_min, y_max - y_min, z_max - z_min])

    _write_descriptor(max_dim,
                      converted_basenames,
                      _descriptor_filename(target_folder,
                                           basename(freecad_filename)))

    # Cleanup of files that are not needed anymore
    if remove_original is True:
        remove(freecad_filename)
    rmtree(folder_unzipping, ignore_errors=True)
