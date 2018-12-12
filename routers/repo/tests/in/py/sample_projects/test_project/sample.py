# coding: utf-8

r"""Example of direct construction"""

from os.path import join, dirname

from osvcad.nodes import Part, Assembly
from osvcad.edges import ConstraintAnchor

# from osvcad.view import view_part, view_assembly, view_assembly_graph

# plate_gn = Part.from_py_script(py_script_path="py_scripts/plate_with_holes.py")
plate_gn = Part.from_py_script(py_script_path=join(dirname(__file__),
                                                   "py_scripts",
                                                   "plate_with_holes.py"))

print("Plate gn : %s" % plate_gn)

screws = [Part.from_library_part(
    library_file_path="libraries/ISO4014_library.json",
    part_id="ISO4014_M2_grade_Bx21") for _ in range(4)]

nuts = [Part.from_library_part(
    library_file_path="libraries/ISO4032_library.json",
    part_id="ISO4032_Nut_M2.0") for _ in range(4)]


assembly = Assembly(root=plate_gn)

for i, screw in enumerate(screws, 1):
    assembly.link(plate_gn,
                  screw,
                  constraint=ConstraintAnchor(anchor_name_master=str(i),
                                              anchor_name_slave=1,
                                              distance=0,
                                              angle=0))

for i, (screw, nut) in enumerate(zip(screws, nuts), 1):
    assembly.link(screw,
                  nut,
                  constraint=ConstraintAnchor(anchor_name_master=1,
                                              anchor_name_slave=1,
                                              distance=-5-1.6,
                                              angle=0))

if __name__ == "__main__":
    print(assembly.number_of_nodes())
    print(assembly.number_of_edges())

    # view_part(nuts[0])
    # view_assembly(assembly)
    # view_assembly_graph(assembly)
