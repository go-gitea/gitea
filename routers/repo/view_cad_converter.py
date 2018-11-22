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
- ** JSON instead of STL -> selectable shapes, faces etc ...

- GITEA_URL should be an environment variable

"""

from __future__ import print_function, absolute_import

import logging
import sys
import zipfile
from os import remove
from os.path import splitext, basename, join, isfile
import json
import hashlib

import uuid

from OCC.Core.Visualization import Tesselator

# from OCC.Extend.TopologyUtils import is_edge, is_wire, discretize_edge, discretize_wire
from TopologyUtils import is_edge, is_wire, discretize_edge, discretize_wire, TopologyExplorer

from requests import get

from aocxchange.step import StepImporter
from aocxchange.iges import IgesImporter
from aocxchange.brep import BrepImporter
from aocxchange.stl import StlExporter, StlImporter

from aocutils.analyze.bounds import BoundingBox

from osvcad.nodes import Part

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
    r"""Build the name of the converted file using the name of the file to be converted

    Parameters
    ----------
    folder : str
        Path to the folder where the converted file is to be written
    name : str
        Base name of the file being converted
    i : int
        Index, as a file may lead to the creation of many converted files if it contains multiple shapes

    Returns
    -------
    str : Path to the converted file

    """
    # return "%s/%s_%i.stl" % (converted_files_folder, name, i)
    # return join(folder, "%s_%i.stl" % (name, i))

    sha1sum = hashlib.sha1()
    with open(join(folder, name), 'rb') as source:
        block = source.read(2 ** 16)
        while len(block) != 0:
            sha1sum.update(block)
            block = source.read(2 ** 16)
    hash_name = sha1sum.hexdigest()
    # return join(folder, "%s_%i.json" % (name, i))
    return join(folder, "%s_%i.json" % (hash_name, i))


def _convert_shape(shape, filename):
    # _convert_shape_stl(shape, filename)
    if isfile(filename):
        pass  # The cached version will be used
    else:
        _shape_to_json(shape, filename)


def color_to_hex(rgb_color):
    """ Takes a tuple with 3 floats between 0 and 1.
    Returns a hex. Useful to convert occ colors to web color code
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
    """
    # first build the array of point coordinates
    # edges are built as follows:
    # points_coordinates  =[P0x, P0y, P0z, P1x, P1y, P1z, P2x, P2y, etc.]
    points_coordinates = []
    for point in point_set:
        for coord in point:
            points_coordinates.append(coord)
    # then build the dictionnary exported to json
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
    r"""Converts a shape to a JSON file representation"""
    _3js_shapes = {}
    _3js_edges = {}

    # if the shape is an edge or a wire, use the related functions
    if is_edge(shape):
        print("discretize an edge")
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
        print("discretize a wire")
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
    _3js_shapes[shape_hash] = [export_edges, color, specular_color, shininess, transparency, line_color, line_width]
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


def _convert_shape_stl(shape, filename):
    r"""Write a shape to the converted file

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
    r"""Build the name of the file that contains the results of the conversion process

    Parameters
    ----------
    converted_files_folder : str
        Path to the folder where the converted files end up
    cad_file_basename : str
        Base name of the CAD file that is being converted

    """
    # return "%s/%s.%s" % (converted_files_folder, cad_file_basename, "dat")
    return join(converted_files_folder, "%s.%s" % (cad_file_basename, "dat"))


def _write_descriptor(max_dim, names, descriptor_filename_):
    with open(descriptor_filename_, 'w') as f:
        f.write("%f\n" % max_dim)
        f.write("\n".join(names))


def convert_freecad_file(freecad_filename, target_folder):
    logger.info("Starting FreeCAD conversion")

    fcstd_as_zip = zipfile.ZipFile(freecad_filename)
    breps_basenames = list(filter(lambda x: splitext(x)[1].lower() in [".brep", ".brp"], fcstd_as_zip.namelist()))

    for brep_basename in breps_basenames:
        fcstd_as_zip.extract(brep_basename, target_folder)

    breps_filenames = ["%s/%s" % (target_folder, name) for name in breps_basenames]
    converted_filenames = [_conversion_filename(target_folder, name, i) for i, name in
                           enumerate(breps_basenames)]
    converted_basenames = [basename(filename) for filename in converted_filenames]

    assert len(breps_basenames) == len(breps_filenames) == len(converted_filenames) == len(converted_basenames)

    extremas = []

    for i, (brep_basename, brep_filename, converted_basename, converted_filename) in enumerate(
            zip(breps_basenames, breps_filenames, converted_basenames, converted_filenames)):

        # fcstd_as_zip.extract(brep_basename, target_folder)

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

    _write_descriptor(max_dim, converted_basenames, _descriptor_filename(target_folder, basename(freecad_filename)))
    remove(freecad_filename)


def convert_step_file(step_filename, target_folder):
    converted_basenames = []
    importer = StepImporter(step_filename)
    shapes = importer.shapes
    max_dim = BoundingBox(importer.compound).max_dimension
    for i, shape in enumerate(shapes):
        converted_filename = _conversion_filename(target_folder, basename(step_filename), i)
        _convert_shape(shape, converted_filename)
        converted_basenames.append(basename(converted_filename))

    _write_descriptor(max_dim, converted_basenames, _descriptor_filename(target_folder, basename(step_filename)))
    remove(step_filename)


def convert_iges_file(iges_filename, target_folder):
    converted_basenames = []
    importer = IgesImporter(iges_filename)
    shapes = importer.shapes
    max_dim = BoundingBox(importer.compound).max_dimension
    for i, shape in enumerate(shapes):
        converted_filename = _conversion_filename(target_folder, basename(iges_filename), i)
        _convert_shape(shape, converted_filename)
        converted_basenames.append(basename(converted_filename))

    _write_descriptor(max_dim, converted_basenames, _descriptor_filename(target_folder, basename(iges_filename)))
    remove(iges_filename)


def convert_brep_file(brep_filename, target_folder):
    shape = BrepImporter(brep_filename).shape
    converted_filename = _conversion_filename(target_folder, basename(brep_filename), 0)
    converted_basenames = [basename(converted_filename)]
    _convert_shape(shape, converted_filename)

    _write_descriptor(BoundingBox(shape).max_dimension, converted_basenames, _descriptor_filename(target_folder, basename(brep_filename)))
    remove(brep_filename)


def convert_stl_file(stl_filename, target_folder):
    importer = StlImporter(stl_filename)
    converted_filename = _conversion_filename(target_folder, basename(stl_filename), 0)
    converted_basenames = [basename(converted_filename)]
    _convert_shape(importer.shape, converted_filename)
    max_dim = BoundingBox(importer.shape).max_dimension
    _write_descriptor(max_dim, converted_basenames, _descriptor_filename(target_folder, basename(stl_filename)))


def convert_py_file(py_filename, target_folder):
    r"""Can contain the definition of a part or of an assembly"""
    import imp

    try:
        part = Part.from_py_script(py_filename)
        shape = part.node_shape.shape
        converted_filename = _conversion_filename(target_folder, basename(py_filename), 0)
        converted_basenames = [basename(converted_filename)]
        _convert_shape(shape, converted_filename)

        _write_descriptor(BoundingBox(shape).max_dimension, converted_basenames, _descriptor_filename(target_folder, basename(py_filename)))
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
            # TODO : max_dim
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

    _write_descriptor(BoundingBox(shape).max_dimension, converted_basenames, _descriptor_filename(target_folder, basename(stepzip_filename)))
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
