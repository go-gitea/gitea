#!/usr/bin/env python
# coding: utf-8

r"""Conversion procedure of Gitea for CAD.

Called from Go code. The parameters are passed as command line arguments
in the Go code.

"""

from __future__ import print_function, absolute_import

import hashlib
import imp
import json
import logging
from os import remove, getcwd, chdir, system
from os.path import splitext, basename, join, isfile, dirname
import sys
import time
import uuid
import zipfile

from OCC.Core.Visualization import Tesselator

from requests import get

from aocxchange.step import StepImporter
from aocxchange.iges import IgesImporter
from aocxchange.brep import BrepImporter
from aocxchange.stl import StlExporter, StlImporter

from aocutils.analyze.bounds import BoundingBox

from osvcad.nodes import Part

# we use our own version of TopologyUtils.py as some functions were not
# available in the OCC that was installed
# from OCC.Extend.TopologyUtils import is_edge, is_wire, discretize_edge, \
#     discretize_wire
from TopologyUtils import is_edge, is_wire, discretize_edge, discretize_wire, \
    TopologyExplorer


# Works in any case : local dev and server
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
        Full path to the local file that is to be created by the download

    Raises
    ------
    requests.exceptions.HTTPError if the URL points to a file
    that does not exist
    FileNotFoundError if the filename cannot be opened

    """
    logger.info("Downloading file at URL : %s" % url)
    response = get(url, stream=True)
    logger.info("Response is %s" % str(response))
    response.raise_for_status()

    with open(filename, 'wb') as f:
        for chunk in response.iter_content(1024):
            f.write(chunk)


def sha1(filename):
    r"""Compute the SHA-1 hash of a file

    Parameters
    ----------
    filename : str
        Full path to file

    Notes
    -----
    The typical call to this function takes 2-3 ms

    Returns
    -------
    str : The SHA-1 has of the file

    Raises
    ------
    FileNotFoundError if filename points to a file that does not exist

    """
    logger.debug("Computing SHA-1 hash of : %s" % filename)
    sha1sum = hashlib.sha1()
    with open(filename, 'rb') as source:
        block = source.read(2 ** 16)
        while len(block) != 0:
            sha1sum.update(block)
            block = source.read(2 ** 16)
    sha1_hash = sha1sum.hexdigest()
    logger.debug("SHA-1 hash is : %s" % sha1_hash)
    return sha1_hash


def _conversion_filename(folder, name, i=0):
    r"""Build the name of the converted file using the name of the file to be
    converted

    Parameters
    ----------
    folder : str
        Path to the folder where the converted file is to be written
    name : str
        Base name of the file being converted
    i : int
        Index, as a file may lead to the creation of many converted files
        if it contains multiple shapes

    Returns
    -------
    str : Path to the converted file

    """
    logger.debug("Call to _conversion_filename()")
    # return join(folder, "%s_%i.stl" % (name, i))

    hash_name = sha1(join(folder, name))
    logger.debug("End of call to _conversion_filename()")
    return join(folder, "%s_%i.json" % (hash_name, i))


def _convert_shape(shape, filename):
    r"""Convert a shape to a format usable for web display.

    The currently used format for web display is JSON

    Parameters
    ----------
    shape : OCC shape
    filename : full path to the target file name

    Returns
    -------
    Nothing, it is a procedure, not a function

    """
    logger.debug("Call to _convert_shape()")
    if isfile(filename):
        logger.info("Using existing file")
        pass  # The cached version will be used
    else:
        logger.info("Converting shape")
        _shape_to_json(shape, filename)
    logger.debug("End of call to _convert_shape()")


def color_to_hex(rgb_color):
    """ Takes a tuple with 3 floats between 0 and 1. Useful to convert occ
    colors to web color code

    Parameters
    ----------
    rgb_color : tuple of floats between 0. and 1.

    Returns
    -------
    Returns a hex.

    """
    r, g, b = rgb_color
    assert 0 <= r <= 1.
    assert 0 <= g <= 1.
    assert 0 <= b <= 1.
    rh = int(r * 255.)
    gh = int(g * 255.)
    bh = int(b * 255.)
    return "0x%.02x%.02x%.02x" % (rh, gh, bh)


def export_edgedata_to_json(edge_hash, point_set):
    """ Export a set of points to a LineSegment buffergeometry

    Parameters
    ----------
    edge_hash
    point_set

    Returns
    -------
    str : a JSON string

    """
    # first build the array of point coordinates
    # edges are built as follows:
    # points_coordinates  =[P0x, P0y, P0z, P1x, P1y, P1z, P2x, P2y, etc.]
    points_coordinates = []
    for point in point_set:
        for coord in point:
            points_coordinates.append(coord)
    # then build the dictionary exported to json
    edges_data = {"metadata": {"version": 4.4,
                               "type": "BufferGeometry",
                               "generator": "pythonocc"},
                  "uuid": edge_hash,
                  "type": "BufferGeometry",
                  "data": {"attributes": {"position": {"itemSize": 3,
                                                       "type": "Float32Array",
                                                       "array": points_coordinates}
                                          }
                           }
                  }
    return json.dumps(edges_data)


# TODO: move control of color and appearance from HTML template to JSON
# TODO : separate the conversion from the IO
def _shape_to_json(shape,
                   filename,
                   export_edges=False,
                   color=(0.65, 0.65, 0.65),
                   specular_color=(1, 1, 1),
                   shininess=0.9,
                   transparency=0.,
                   line_color=(0, 0., 0.),
                   line_width=2.,
                   mesh_quality=1.):
    r"""Converts a shape to a JSON file representation

    Parameters
    ----------
    shape : OCC shape
    filename : str
        Full path to the file where the conversion is written
    export_edges : bool
    color
    specular_color
    shininess
    transparency
    line_color
    line_width
    mesh_quality

    Returns
    -------
    Nothing, it is a procedure, not a function

    """
    logger.info("Starting the conversion of a shape to JSON(%s)" % filename)
    _3js_shapes = {}
    _3js_edges = {}

    # if the shape is an edge or a wire, use the related functions
    if is_edge(shape):
        logger.debug("discretize an edge")
        pnts = discretize_edge(shape)
        edge_hash = "edg%s" % uuid.uuid4().hex
        str_to_write = export_edgedata_to_json(edge_hash, pnts)
        # edge_full_path = os.path.join(path, edge_hash + '.json')
        with open(filename, "w") as edge_file:
            edge_file.write(str_to_write)
        # store this edge hash
        _3js_edges[edge_hash] = [color, line_width]
        return True

    elif is_wire(shape):
        logger.debug("discretize a wire")
        pnts = discretize_wire(list(TopologyExplorer(shape).wires())[0])
        wire_hash = "wir%s" % uuid.uuid4().hex
        str_to_write = export_edgedata_to_json(wire_hash, pnts)
        # wire_full_path = os.path.join(path, wire_hash + '.json')
        with open(filename, "w") as wire_file:
            wire_file.write(str_to_write)
        # store this edge hash
        _3js_edges[wire_hash] = [color, line_width]
        return True

    shape_uuid = uuid.uuid4().hex
    shape_hash = "shp%s" % shape_uuid
    # tesselate
    tess = Tesselator(shape)
    tess.Compute(compute_edges=export_edges,
                 mesh_quality=mesh_quality,
                 uv_coords=False,
                 parallel=True)

    # export to 3JS
    # shape_full_path = os.path.join(path, shape_hash + '.json')
    # add this shape to the shape dict, sotres everything related to it
    _3js_shapes[shape_hash] = [export_edges,
                               color,
                               specular_color,
                               shininess,
                               transparency,
                               line_color,
                               line_width]
    # generate the mesh
    # tess.ExportShapeToThreejs(shape_hash, shape_full_path)
    # and also to JSON
    with open(filename, 'w') as json_file:
        json_file.write(tess.ExportShapeToThreejsJSONString(shape_uuid))

    # draw edges if necessary
    # if export_edges:
    #     # export each edge to a single json
    #     # get number of edges
    #     nbr_edges = tess.ObjGetEdgeCount()
    #     for i_edge in range(nbr_edges):
    #         # after that, the file can be appended
    #         str_to_write = ''
    #         edge_point_set = []
    #         nbr_vertices = tess.ObjEdgeGetVertexCount(i_edge)
    #         for i_vert in range(nbr_vertices):
    #             edge_point_set.append(tess.GetEdgeVertex(i_edge, i_vert))
    #         # write to file
    #         edge_hash = "edg%s" % uuid.uuid4().hex
    #         str_to_write += export_edgedata_to_json(edge_hash, edge_point_set)
    #         # create the file
    #         edge_full_path = os.path.join(path, edge_hash + '.json')
    #         with open(edge_full_path, "w") as edge_file:
    #             edge_file.write(str_to_write)
    #         # store this edge hash, with black color
    #         _3js_edges[hash] = [(0, 0, 0), line_width]
    logger.info("End of the conversion of a shape to JSON(%s)" % filename)


def _convert_shape_stl(shape, filename):
    r"""Write a shape to the converted file

    NOT USED ANYMORE - DEPRECATED

    Parameters
    ----------
    shape : OCC Shape
        The input shape
    filename : str
        Path to the destination file

    """
    e = StlExporter(filename=filename, ascii_mode=False)
    e.set_shape(shape)
    e.write_file()


def _descriptor_filename(converted_files_folder, cad_file_basename):
    r"""Build the name of the file that contains the results of the
    conversion process

    Parameters
    ----------
    converted_files_folder : str
        Path to the folder where the converted files end up
    cad_file_basename : str
        Base name of the CAD file that is being converted

    Returns
    -------
    str : full path to the descriptor file

    """
    return join(converted_files_folder, "%s.%s" % (cad_file_basename, "dat"))


def _write_descriptor(max_dim, names, descriptor_filename_):
    r"""Write the contents of a descriptor file

    Parameters
    ----------
    max_dim : float
        Maximum dimension of the bounding box of all objects in the CAD file
    names : list[str]
        Names of all files that should be loaded by three.js to build the
        web display of the CAD file

    Returns
    -------
    Nothing, it is a procedure

    """
    with open(descriptor_filename_, 'w') as f:
        f.write("%f\n" % max_dim)
        f.write("\n".join(names))


# TODO : clean breps extracted from zip
def convert_freecad_file(freecad_filename, target_folder):
    r"""Convert a FreeCAD file (.fcstd) for web display

    Parameters
    ----------
    freecad_filename : str
        Full path to FreeCAD file
    target_folder : str
        Full path to the target folder for the conversion

    Returns
    -------
    Nothing, it is a procedure

    """
    logger.info("Starting FreeCAD conversion")

    fcstd_as_zip = zipfile.ZipFile(freecad_filename)
    breps_basenames = list(filter(lambda x: splitext(x)[1].lower() in [".brep",
                                                                       ".brp"],
                                  fcstd_as_zip.namelist()))

    for brep_basename in breps_basenames:
        fcstd_as_zip.extract(brep_basename, target_folder)

    breps_filenames = ["%s/%s" % (target_folder, name)
                       for name in breps_basenames]
    converted_filenames = [_conversion_filename(target_folder, name, i)
                           for i, name in enumerate(breps_basenames)]

    converted_basenames = [basename(filename)
                           for filename in converted_filenames]

    assert len(breps_basenames) == len(breps_filenames) == \
        len(converted_filenames) == len(converted_basenames)

    extremas = []

    for i, (brep_basename,
            brep_filename,
            converted_basename,
            converted_filename) \
            in enumerate(zip(breps_basenames,
                             breps_filenames,
                             converted_basenames,
                             converted_filenames)):

        try:
            importer = BrepImporter(brep_filename)
            extremas.append(BoundingBox(importer.shape).as_tuple)
            _convert_shape(importer.shape, converted_filename)
        except RuntimeError:
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
    remove(freecad_filename)


def convert_step_file(step_filename, target_folder):
    r"""Convert a STEP file (.step, .stp) for web display

    Parameters
    ----------
    step_filename : str
        Full path to the STEP file
    target_folder : str
        Full path to the target folder for the conversion

    Returns
    -------
    Nothing, it is a procedure

    """
    converted_basenames = []
    importer = StepImporter(step_filename)
    shapes = importer.shapes
    max_dim = BoundingBox(importer.compound).max_dimension
    for i, shape in enumerate(shapes):
        converted_filename = _conversion_filename(target_folder,
                                                  basename(step_filename),
                                                  i)
        _convert_shape(shape, converted_filename)
        converted_basenames.append(basename(converted_filename))

    _write_descriptor(max_dim,
                      converted_basenames,
                      _descriptor_filename(target_folder,
                                           basename(step_filename)))
    remove(step_filename)


def convert_iges_file(iges_filename, target_folder):
    r"""Convert an IGES file (.iges, .igs) for web display

    Parameters
    ----------
    iges_filename : str
        Full path to IGES file
    target_folder : str
        Full path to the target folder for the conversion

    Returns
    -------
    Nothing, it is a procedure

    """
    converted_basenames = []
    importer = IgesImporter(iges_filename)
    shapes = importer.shapes
    max_dim = BoundingBox(importer.compound).max_dimension
    for i, shape in enumerate(shapes):
        converted_filename = _conversion_filename(target_folder,
                                                  basename(iges_filename),
                                                  i)
        _convert_shape(shape, converted_filename)
        converted_basenames.append(basename(converted_filename))

    _write_descriptor(max_dim,
                      converted_basenames,
                      _descriptor_filename(target_folder,
                                           basename(iges_filename)))
    remove(iges_filename)


def convert_brep_file(brep_filename, target_folder):
    r"""Convert a BREP file (.brep, .brp) for web display

    Parameters
    ----------
    brep_filename : str
        Full path to the BREP file
    target_folder : str
        Full path to the target folder for the conversion

    Returns
    -------
    Nothing, it is a procedure

    """
    shape = BrepImporter(brep_filename).shape
    converted_filename = _conversion_filename(target_folder,
                                              basename(brep_filename),
                                              0)
    converted_basenames = [basename(converted_filename)]
    _convert_shape(shape, converted_filename)

    _write_descriptor(BoundingBox(shape).max_dimension,
                      converted_basenames,
                      _descriptor_filename(target_folder,
                                           basename(brep_filename)))
    remove(brep_filename)


def convert_stl_file(stl_filename, target_folder):
    r"""Convert a STL file (.stl) for web display

    Parameters
    ----------
    stl_filename : str
        Full path to the STL file
    target_folder : str
        Full path to the target folder for the conversion

    Returns
    -------
    Nothing, it is a procedure

    """
    importer = StlImporter(stl_filename)
    converted_filename = _conversion_filename(target_folder,
                                              basename(stl_filename),
                                              0)
    converted_basenames = [basename(converted_filename)]
    _convert_shape(importer.shape, converted_filename)
    max_dim = BoundingBox(importer.shape).max_dimension
    _write_descriptor(max_dim,
                      converted_basenames,
                      _descriptor_filename(target_folder,
                                           basename(stl_filename)))


def convert_py_file_part(py_filename, target_folder):
    r"""Convert an OsvCad Python file that contains a part for web display

    The Python file contains the definition of a part.

    Parameters
    ----------
    py_filename : str
        Full path to the Python file
    target_folder : str
        Full path to the target folder for the conversion

    Returns
    -------
    Nothing, it is a procedure

    Raises
    ------
    ValueError if not a part definition

    """
    part = Part.from_py_script(py_filename)
    shape = part.node_shape.shape
    converted_filename = _conversion_filename(target_folder,
                                              basename(py_filename),
                                              0)
    converted_basenames = [basename(converted_filename)]
    _convert_shape(shape, converted_filename)

    _write_descriptor(BoundingBox(shape).max_dimension,
                      converted_basenames,
                      _descriptor_filename(target_folder,
                                           basename(py_filename)))
    remove(py_filename)


def convert_py_file_assembly(py_filename, target_folder, clone_url, branch):
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
        converted_filename = _conversion_filename(target_folder,
                                                  basename(py_filename),
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

    # TODO : remove folder created by git clone or the next clone will fail


def convert_py_file(py_filename, target_folder, clone_url, branch):
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
        convert_py_file_part(py_filename, target_folder)
    except (ValueError, FileNotFoundError, ImportError):
        # probably no part attribute in module
        msg = "No part attribute in module"
        logger.warning(msg)
        try:
            convert_py_file_assembly(py_filename,
                                     target_folder,
                                     clone_url,
                                     branch)

        except AttributeError:
            msg = "No part nor assembly attribute in module"
            logger.error(msg)


# TODO : how to display anchors?
def convert_stepzip_file(stepzip_filename, target_folder):
    r"""Convert an OsvCad Stepzip file for web display

    A Stepzip file contains a STEP geometry file and an anchors definition file
    zipped together

    Parameters
    ----------
    stepzip_filename : str
        Full path to the Stepzip file
    target_folder : str
        Full path to the target folder for the conversion

    Returns
    -------
    Nothing, it is a procedure

    """
    part = Part.from_stepzip(stepzip_filename)
    shape = part.node_shape.shape
    converted_filename = _conversion_filename(target_folder,
                                              basename(stepzip_filename),
                                              0)
    converted_basenames = [basename(converted_filename)]
    _convert_shape(shape, converted_filename)

    _write_descriptor(BoundingBox(shape).max_dimension,
                      converted_basenames,
                      _descriptor_filename(target_folder,
                                           basename(stepzip_filename)))
    remove(stepzip_filename)


def main():
    r"""Procedure that handles the conversion from a CAD file to a format
    usable by a 3D web viewer

    The parameters are command line arguments retrieved in this procedure

    """

    t0 = time.time()
    logging.basicConfig(level=logging.DEBUG,
                        format='%(asctime)s :: %(levelname)8s :: %(module)20s '
                               ':: %(lineno)3d :: %(message)s')

    # Retrieve parameters from command
    # cad_file_raw_url = sys.argv[1]  # Direct download URL for the file
    raw_link = sys.argv[1]
    tree_path = sys.argv[2]
    cad_file_raw_url = "%s/%s" % (raw_link, tree_path)
    converted_files_folder = sys.argv[3]  # Root destination for converted files

    branch = raw_link.split("/")[-1]
    user = raw_link.split("/")[1]
    project = raw_link.split("/")[2]
    clone_url = "%s/%s/%s" % (GITEA_URL, user, project)

    logger.info("raw_link is %s" % raw_link)
    logger.info("Branch is %s" % branch)
    logger.info("User is %s" % user)
    logger.info("Project is %s" % project)

    cad_file_raw_url_full = "%s%s" % (GITEA_URL, cad_file_raw_url)

    cad_file_filename = join(converted_files_folder, basename(cad_file_raw_url))

    # Download the original CAD file to the converted files folder
    # (it will be deleted after conversion)
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
        if splitext(cad_file_raw_url)[1].lower() != ".py":
            conversion_function[splitext(cad_file_raw_url)[1].lower()](
                cad_file_filename, converted_files_folder)
        else:
            logger.info("Dealing with a Python file")
            logger.debug("cad_file_filename : %s" % cad_file_filename)
            logger.debug("converted_files_folder : %s" % converted_files_folder)
            logger.debug("clone_url : %s" % clone_url)
            logger.debug("branch : %s" % branch)
            convert_py_file(cad_file_filename,
                            converted_files_folder,
                            clone_url,
                            branch)
        t1 = time.time()
        logger.info("The whole Python call took %f" % (t1 - t0))
        sys.exit(0)
    except KeyError:
        msg = "Unknown CAD cad_file_extension : %s" % cad_file_extension
        logger.error(msg)
        t1 = time.time()
        logger.info("The whole Python call took %f" % (t1 - t0))
        raise ValueError(msg)


if __name__ == "__main__":
    main()
