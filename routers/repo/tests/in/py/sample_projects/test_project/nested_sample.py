# coding: utf-8

r"""Example of direct construction"""

from osvcad.nodes import Part, Assembly
from osvcad.edges import ConstraintAnchor

from osvcad.view import view_part, view_assembly, view_assembly_graph

plate_gn = Part.from_py_script(py_script_path="py_scripts/plate_with_holes.py")

print("Plate gn : %s" % plate_gn)

screws = [Part.from_library_part(
    library_file_path="libraries/ISO4014_library.json",
    part_id="ISO4014_M2_grade_Bx21") for _ in range(4)]

nuts = [Part.from_library_part(
    library_file_path="libraries/ISO4032_library.json",
    part_id="ISO4032_Nut_M2.0") for _ in range(4)]


A = Assembly(root=plate_gn)
project = Assembly(root=A)

for i in range(4):
    bolt = Assembly(root=screws[i])
    bolt.link(screws[i],
              nuts[i],
              constraint=ConstraintAnchor(anchor_name_master=1,
                                          anchor_name_slave=1,
                                          distance=-5-1.6,
                                          angle=0))

    project.link(A,
                 bolt,
                 constraint=ConstraintAnchor(anchor_name_master=str(hash(plate_gn)) + "/%i" % (i + 1),
                                             anchor_name_slave=str(hash(screws[i])) + "/1",
                                             distance=0,
                                             angle=0))

if __name__ == "__main__":
    view_assembly(A)
    view_assembly(project)
    view_assembly_graph(project)
