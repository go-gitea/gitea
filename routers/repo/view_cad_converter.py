#!/opt/miniconda3/bin/python3

r"""Conversion procedure of Gitea for CAD.

Called from Go code. The parameters are passed as command line arguments in the Go code.

TODO:

- osvcad files display
  + ******** parts (ok minus anchors)
  + ******** stepzips (ok minus anchors)
  + assemblies (how to deal with imports and relative filepaths?)
  + anchors display
- hash based filename
- cache based on hash
- JSON instead of STL -> selectable shapes, faces etc ...

- GITEA_URL should be an environment variable

"""

import logging
import sys
import zipfile
from os import remove
from os.path import splitext, basename, join

from requests import get

from aocxchange.step import StepImporter
from aocxchange.iges import IgesImporter
from aocxchange.brep import BrepImporter
from aocxchange.stl import StlExporter

from osvcad.nodes import Part

GITEA_URL = "http://localhost:3000"
# GITEA_URL = "http://127.0.0.1:3000"

logger = logging.getLogger(__name__)


def _download_file(url, filename):
    r"""Download an external file (at specified url) to a local file

    Parameters
    ----------
    url : str
        URL to the file to be downloaded
    filename : str
        Full path to the local file

    """
    logger.info("Downloading file at URL : %s" % url)
    response = get(url, stream=True)
    logger.info("Response is %s" % str(response))
    response.raise_for_status()

    with open(filename, 'wb') as f:
        for chunk in response.iter_content(1024):
            f.write(chunk)


def _conversion_filename(folder, name, i=0):
    # return "%s/%s_%i.stl" % (converted_files_folder, name, i)
    return join(folder, "%s_%i.stl" % (name, i))


def _convert_shape(shape, filename):
    e = StlExporter(filename=filename, ascii_mode=False)
    e.set_shape(shape)
    e.write_file()


def _descriptor_filename(converted_files_folder, cad_file_basename):
    # return "%s/%s.%s" % (converted_files_folder, cad_file_basename, "dat")
    return join(converted_files_folder, "%s.%s" % (cad_file_basename, "dat"))


def _write_descriptor(names, descriptor_filename_):
    with open(descriptor_filename_, 'w') as f:
        f.write("\n".join(names))


def convert_freecad_file(freecad_filename, target_folder):
    logger.info("Starting FreeCAD conversion")

    fcstd_as_zip = zipfile.ZipFile(freecad_filename)

    breps_basenames = list(filter(lambda x: splitext(x)[1].lower() in [".brep", ".brp"], fcstd_as_zip.namelist()))
    breps_filenames = ["%s/%s" % (target_folder, name) for name in breps_basenames]
    converted_filenames = [_conversion_filename(target_folder, name, i) for i, name in
                           enumerate(breps_basenames)]
    converted_basenames = [basename(filename) for filename in converted_filenames]

    assert len(breps_basenames) == len(breps_filenames) == len(converted_filenames) == len(converted_basenames)

    for i, (brep_basename, brep_filename, converted_basename, converted_filename) in enumerate(
            zip(breps_basenames, breps_filenames, converted_basenames, converted_filenames)):

        fcstd_as_zip.extract(brep_basename, target_folder)

        try:
            _convert_shape(BrepImporter(brep_filename).shape, converted_filename)
        except RuntimeError:
            logger.error("RuntimeError for %s" % brep_filename)

    _write_descriptor(converted_basenames, _descriptor_filename(target_folder, basename(freecad_filename)))
    remove(freecad_filename)


def convert_step_file(step_filename, target_folder):
    converted_basenames = []
    shapes = StepImporter(step_filename).shapes
    for i, shape in enumerate(shapes):
        converted_filename = _conversion_filename(target_folder, basename(step_filename), i)
        _convert_shape(shape, converted_filename)
        converted_basenames.append(basename(converted_filename))

    _write_descriptor(converted_basenames, _descriptor_filename(target_folder, basename(step_filename)))
    remove(step_filename)


def convert_iges_file(iges_filename, target_folder):
    converted_basenames = []
    shapes = IgesImporter(iges_filename).shapes
    for i, shape in enumerate(shapes):
        converted_filename = _conversion_filename(target_folder, basename(iges_filename), i)
        _convert_shape(shape, converted_filename)
        converted_basenames.append(basename(converted_filename))

    _write_descriptor(converted_basenames, _descriptor_filename(target_folder, basename(iges_filename)))
    remove(iges_filename)


def convert_brep_file(brep_filename, target_folder):
    shape = BrepImporter(brep_filename).shape
    converted_filename = _conversion_filename(target_folder, basename(brep_filename), 0)
    converted_basenames = [basename(converted_filename)]
    _convert_shape(shape, converted_filename)

    _write_descriptor(converted_basenames, _descriptor_filename(target_folder, basename(brep_filename)))
    remove(brep_filename)


def convert_stl_file(stl_filename, target_folder):
    converted_basenames = [basename(stl_filename)]
    _write_descriptor(converted_basenames, _descriptor_filename(target_folder, basename(stl_filename)))


def convert_py_file(py_filename, target_folder):
    r"""Can contain the definition of a part or of an assembly"""
    import imp

    try:
        part = Part.from_py_script(py_filename)
        shape = part.node_shape.shape
        converted_filename = _conversion_filename(target_folder, basename(py_filename), 0)
        converted_basenames = [basename(converted_filename)]
        _convert_shape(shape, converted_filename)

        _write_descriptor(converted_basenames, _descriptor_filename(target_folder, basename(py_filename)))
        remove(py_filename)
    except ValueError:  # probably no part attribute in module
        msg = "No part attribute in module"
        logger.warning(msg)
        try:
            # TODO : FIXME
            # Does not work because of import paths
            converted_basenames = []
            module_ = imp.load_source(splitext(basename(py_filename))[0],
                                      py_filename)
            assembly = getattr(module_, "assembly")

            for i, node in enumerate(assembly.nodes()):
                shape = node.node_shape.shape
                converted_filename = _conversion_filename(target_folder, basename(py_filename), i)
                converted_basenames.append(basename(converted_filename))
                _convert_shape(shape, converted_filename)
            _write_descriptor(converted_basenames, _descriptor_filename(target_folder, basename(py_filename)))
            remove(py_filename)
        except AttributeError:
            msg = "No part nor assembly attribute in module"
            logger.error(msg)


def convert_stepzip_file(stepzip_filename, target_folder):
    # TODO : how to display anchors?
    part = Part.from_stepzip(stepzip_filename)
    shape = part.node_shape.shape
    converted_filename = _conversion_filename(target_folder, basename(stepzip_filename), 0)
    converted_basenames = [basename(converted_filename)]
    _convert_shape(shape, converted_filename)

    _write_descriptor(converted_basenames, _descriptor_filename(target_folder, basename(stepzip_filename)))
    remove(stepzip_filename)


def main():
    r"""Procedure that handles the conversion from a CAD file to a format usable by a 3D web viewer

    The parameters are command line arguments retrieved in this procedure

    """
    logging.basicConfig(level=logging.DEBUG,
                        format='%(asctime)s :: %(levelname)8s :: %(module)20s '
                               ':: %(lineno)3d :: %(message)s')

    # Retrieve parameters from command
    cad_file_raw_url = sys.argv[1]  # Direct download URL for the file
    converted_files_folder = sys.argv[2]  # Root destination for converted files

    cad_file_raw_url_full = "%s%s" % (GITEA_URL, cad_file_raw_url)

    cad_file_filename = join(converted_files_folder, basename(cad_file_raw_url))

    # Download the original CAD file to the converted files folder (it will be deleted after conversion)
    _download_file(cad_file_raw_url_full, cad_file_filename)

    cad_file_extension = splitext(cad_file_raw_url)[1]

    conversion_function = {".fcstd": convert_freecad_file,
                           ".step": convert_step_file,
                           ".stp": convert_step_file,
                           ".iges": convert_iges_file,
                           ".igs": convert_iges_file,
                           ".brep": convert_brep_file,
                           ".brp": convert_brep_file,
                           ".stl": convert_stl_file,
                           ".py": convert_py_file,
                           ".stepzip": convert_stepzip_file}

    try:
        conversion_function[splitext(cad_file_raw_url)[1].lower()](cad_file_filename, converted_files_folder)
        sys.exit(0)
    except KeyError:
        msg = "Unknown CAD cad_file_extension : %s" % cad_file_extension
        logger.error(msg)
        raise ValueError(msg)


main()
