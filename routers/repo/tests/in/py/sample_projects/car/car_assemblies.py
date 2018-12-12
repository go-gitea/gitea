# coding: utf-8

r"""Example of a car model"""

# import logging

from osvcad.nodes import Part, Assembly
from osvcad.edges import ConstraintAnchor


def make_chassis_assembly():
    r"""Chassis assembly creation"""

    p1_base = Part.from_stepzip(
        stepzip_file="shelf/chassis/CAR-CHASSIS-BASE-2.38_0.179_1.18-STEEL--.stepzip")
    p2_l = Part.from_stepzip(
        stepzip_file="shelf/chassis/"
                     "CAR-CHASSIS-ARCHLEFT-705_515_184_mm-STEEL--.stepzip")
    p2_r = Part.from_stepzip(
        stepzip_file="shelf/chassis/"
                     "CAR-CHASSIS-ARCHRIGHT-705_515_184_mm-STEEL--.stepzip")
    p4 = Part.from_stepzip(
        stepzip_file="shelf/chassis/"
                     "CAR-CHASSIS-ARCHSTRUT-127_126_796_mm-STEEL--.stepzip")
    p5 = Part.from_stepzip(
        stepzip_file="shelf/chassis/"
                     "CAR-CHASSIS-SEATSSUPPORT-410_151_1174_mm-STEEL--.stepzip")
    p6 = Part.from_stepzip(
        stepzip_file="shelf/chassis/"
                     "CAR-CHASSIS-DASHBOARDSUPPORT-107_535_1184_mm-STEEL--.stepzip")
    p7_l = Part.from_stepzip(
        stepzip_file="shelf/chassis/"
                     "CAR-SUSPENSION-ARCHLEFT-526_535_284_mm-STEEL--.stepzip")
    p7_r = Part.from_stepzip(
        stepzip_file="shelf/chassis/"
                     "CAR-SUSPENSION-ARCHRIGHT-526_535_284_mm-STEEL--.stepzip")
    p8 = Part.from_stepzip(
        stepzip_file="shelf/chassis/"
                     "CAR-CHASSIS-ARCHSTRUT-111_130_746_mm-STEEL--.stepzip")
    p9 = Part.from_stepzip(
        stepzip_file="shelf/chassis/"
                     "CAR-CHASSIS-DASHBOARDSUPPORTREINFORCEMENT-205_525_75_mm-STEEL--.stepzip")

    chassis_assembly = Assembly(root=p1_base)

    chassis_assembly.link(p1_base, p2_l, constraint=ConstraintAnchor(
        anchor_name_master="A2-L",
        anchor_name_slave="D3",
        distance=0,
        angle=0))

    chassis_assembly.link(p1_base, p2_r, constraint=ConstraintAnchor(
        anchor_name_master="A2-R",
        anchor_name_slave="D3",
        distance=0,
        angle=0))

    chassis_assembly.link(p2_r, p4, constraint=ConstraintAnchor(
        anchor_name_master="B2",
        anchor_name_slave="B4",
        distance=0,
        angle=0))

    chassis_assembly.link(p1_base, p5, constraint=ConstraintAnchor(
        anchor_name_master="F2-R",
        anchor_name_slave="F1",
        distance=0,
        angle=0))

    chassis_assembly.link(p1_base, p6, constraint=ConstraintAnchor(
        anchor_name_master="G3-L",
        anchor_name_slave="A1",
        distance=0,
        angle=0))

    chassis_assembly.link(p1_base, p7_l, constraint=ConstraintAnchor(
        anchor_name_master="K3-L",
        anchor_name_slave="A4",
        distance=0,
        angle=0))

    chassis_assembly.link(p1_base, p7_r, constraint=ConstraintAnchor(
        anchor_name_master="K3-R",
        anchor_name_slave="A4",
        distance=0,
        angle=0))

    chassis_assembly.link(p7_l, p8, constraint=ConstraintAnchor(
        anchor_name_master="B1",
        anchor_name_slave="A1",
        distance=0,
        angle=0))

    chassis_assembly.link(p1_base, p9, constraint=ConstraintAnchor(
        anchor_name_master="H2",
        anchor_name_slave="A1",
        distance=0,
        angle=0))

    return chassis_assembly


def make_front_suspension_assembly():
    r"""Front suspension assembly creation"""
    p1 = [Part.from_stepzip(
        "shelf/suspension/common/"
        "CAR-SUSPENSION-BEARING-l54.7_d37_mm---.stepzip") for _ in range(2)]

    p2 = Part.from_stepzip(
        "shelf/suspension/front/"
        "CAR-SUSPENSION-FORK-320_44_270_mm---.stepzip")
    p3 = Part.from_stepzip(
        "shelf/suspension/front/"
        "CAR-SUSPENSION-LINK-28_23_124_mm---.stepzip")
    p4 = Part.from_stepzip(
        "shelf/suspension/front/"
        "CAR-DIRECTION-BALLHEAD-D23_d10_l70_mm---.stepzip")
    p5 = Part.from_stepzip(
        "shelf/suspension/front/"
        "CAR-SUSPENSION-HUB-107_212_84_mm---.stepzip")

    p6 = Part.from_stepzip(
        "shelf/suspension/common/"
        "CAR-SUSPENSION-DISCSUPPORT-117_117_70_mm---.stepzip")
    p7 = Part.from_stepzip(
        "shelf/suspension/common/"
        "CAR-AXLE-DISC-d227_h46_mm-STEEL--.stepzip",
        instance_id="P7_Front")
    p8 = Part.from_stepzip(
        "shelf/suspension/common/"
        "CAR-SUSPENSION-CYLINDER-l320_d42---.stepzip")
    p9 = Part.from_stepzip(
        "shelf/suspension/common/"
        "CAR-SUSPENSION-PISTON-l381_d33_d16-STEEL--.stepzip")
    p10 = Part.from_stepzip(
        "shelf/suspension/common/"
        "CAR-SUSPENSION-HAT-102_40_70_mm---.stepzip")
    p11 = Part.from_stepzip(
        "shelf/suspension/common/"
        "CAR-SUSPENSION-HEAD-60_48_67_mm---.stepzip")
    p12 = Part.from_stepzip(
        "shelf/suspension/common/"
        "CAR-SUSPENSION-NECK-d28_l51_mm---.stepzip")

    front_suspension_assembly = Assembly(root=p2)

    front_suspension_assembly.link(p2, p1[0], constraint=ConstraintAnchor(
        anchor_name_master="out1",
        anchor_name_slave="wide_out",
        distance=0,
        angle=0))

    front_suspension_assembly.link(p2, p1[1], constraint=ConstraintAnchor(
        anchor_name_master="out2",
        anchor_name_slave="wide_out",
        distance=0,
        angle=0))

    front_suspension_assembly.link(p2, p3, constraint=ConstraintAnchor(
        anchor_name_master="in_inside",
        anchor_name_slave="main",
        distance=-71.396,
        angle=0))

    front_suspension_assembly.link(p3, p4, constraint=ConstraintAnchor(
        anchor_name_master="perp",
        anchor_name_slave="cone",
        distance=6.2,
        angle=0))

    front_suspension_assembly.link(p4, p5, constraint=ConstraintAnchor(
        anchor_name_master="ball",
        anchor_name_slave="ball",
        distance=0,
        angle=0))

    front_suspension_assembly.link(p5, p8, constraint=ConstraintAnchor(
        anchor_name_master="side1_top",
        anchor_name_slave="side2_top",
        distance=0,
        angle=-14.566))

    front_suspension_assembly.link(p8, p9, constraint=ConstraintAnchor(
        anchor_name_master="top",
        anchor_name_slave="bottom",
        distance=-216.148,
        angle=0))

    front_suspension_assembly.link(p9, p12, constraint=ConstraintAnchor(
        anchor_name_master="top",
        anchor_name_slave="bottom",
        distance=1.24,
        angle=0))

    front_suspension_assembly.link(p12, p11, constraint=ConstraintAnchor(
        anchor_name_master="bottom",
        anchor_name_slave="bottom",
        distance=0,
        angle=0))

    front_suspension_assembly.link(p11, p10, constraint=ConstraintAnchor(
        anchor_name_master="wide_flat",
        anchor_name_slave="axis_bottom",
        distance=0,
        angle=0))

    # TODO : create a way to position p6 on p5 and p7 on p6 so that the
    #        holes are in front of one another without requiring
    #        a 'magic' angle value

    front_suspension_assembly.link(p5, p6, constraint=ConstraintAnchor(
        anchor_name_master="wheel_axis",
        anchor_name_slave="axis_drive",
        distance=0,
        angle=0))

    front_suspension_assembly.link(p6, p7, constraint=ConstraintAnchor(
        anchor_name_master="axis_disc",
        anchor_name_slave="inside",
        distance=0,
        angle=0))

    return front_suspension_assembly


def make_rear_suspension_assembly():
    r"""Rear suspension assembly creation"""
    p1 = [Part.from_stepzip(
        "shelf/suspension/common/"
        "CAR-SUSPENSION-BEARING-l54.7_d37_mm---.stepzip") for _ in range(4)]
    p2 = Part.from_stepzip(
        "shelf/suspension/rear/"
        "CAR-SUSPENSION-FRAME-320_49_327_mm---.stepzip")
    p5 = Part.from_stepzip(
        "shelf/suspension/rear/"
        "CAR-SUSPENSION-HUB-200_240_82_mm---.stepzip")
    p7 = Part.from_stepzip(
        "shelf/suspension/common/"
        "CAR-AXLE-DISC-d227_h46_mm-STEEL--.stepzip",
        instance_id="P7_Rear")
    p8 = Part.from_stepzip(
        "shelf/suspension/common/"
        "CAR-SUSPENSION-CYLINDER-l320_d42---.stepzip")
    p9 = Part.from_stepzip(
        "shelf/suspension/common/"
        "CAR-SUSPENSION-PISTON-l381_d33_d16-STEEL--.stepzip")
    p10 = Part.from_stepzip(
        "shelf/suspension/common/"
        "CAR-SUSPENSION-HAT-102_40_70_mm---.stepzip")
    p11 = Part.from_stepzip(
        "shelf/suspension/common/"
        "CAR-SUSPENSION-HEAD-60_48_67_mm---.stepzip")
    p12 = Part.from_stepzip(
        "shelf/suspension/common/"
        "CAR-SUSPENSION-NECK-d28_l51_mm---.stepzip")

    rear_suspension_assembly = Assembly(root=p2)

    rear_suspension_assembly.link(p2, p1[0], constraint=ConstraintAnchor(
        anchor_name_master="out1",
        anchor_name_slave="wide_out",
        distance=0,
        angle=0))

    rear_suspension_assembly.link(p2, p1[1], constraint=ConstraintAnchor(
        anchor_name_master="out2",
        anchor_name_slave="wide_out",
        distance=0,
        angle=0))

    rear_suspension_assembly.link(p2, p1[2], constraint=ConstraintAnchor(
        anchor_name_master="in1",
        anchor_name_slave="wide_out",
        distance=0,
        angle=0))

    rear_suspension_assembly.link(p2, p1[3], constraint=ConstraintAnchor(
        anchor_name_master="in2",
        anchor_name_slave="wide_out",
        distance=0,
        angle=0))

    rear_suspension_assembly.link(p1[3], p5, constraint=ConstraintAnchor(
        anchor_name_master="narrow_out",
        anchor_name_slave="bottom2",
        distance=0,
        angle=0))

    rear_suspension_assembly.link(p5, p7, constraint=ConstraintAnchor(
        anchor_name_master="wheel_axis",
        anchor_name_slave="inside",
        distance=62,
        angle=0))

    rear_suspension_assembly.link(p5, p8, constraint=ConstraintAnchor(
        anchor_name_master="side1_top",
        anchor_name_slave="side1_top",
        distance=0,
        angle=14.566))

    rear_suspension_assembly.link(p8, p9, constraint=ConstraintAnchor(
        anchor_name_master="top",
        anchor_name_slave="bottom",
        distance=-216.148,
        angle=0))

    rear_suspension_assembly.link(p9, p12, constraint=ConstraintAnchor(
        anchor_name_master="top",
        anchor_name_slave="bottom",
        distance=1.24,
        angle=0))

    rear_suspension_assembly.link(p12, p11, constraint=ConstraintAnchor(
        anchor_name_master="bottom",
        anchor_name_slave="bottom",
        distance=0,
        angle=0))

    rear_suspension_assembly.link(p11, p10, constraint=ConstraintAnchor(
        anchor_name_master="wide_flat",
        anchor_name_slave="axis_bottom",
        distance=0,
        angle=0))

    return rear_suspension_assembly


def make_wheel_assembly():
    r"""Wheel assembly creation"""
    rim = Part.from_stepzip(
        stepzip_file="shelf/wheel/"
                     "CAR-WHEEL-RIM-D416_l174_mm---.stepzip",
        instance_id="rim")
    tyre = Part.from_stepzip(
        stepzip_file="shelf/wheel/CAR-WHEEL-TYRE-D575_l178_mm-RUBBER--.stepzip")

    wheel_assembly = Assembly(root=rim)

    wheel_assembly.link(rim,
                        tyre,
                        constraint=ConstraintAnchor(anchor_name_master="AXIS_TYRE_d412#mm_",
                                                    anchor_name_slave="AXIS_SIDE_d383#mm_",
                                                    distance=0,
                                                    angle=0))

    return wheel_assembly
