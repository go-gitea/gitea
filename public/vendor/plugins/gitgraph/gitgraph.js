/*
 * @license magnet:?xt=urn:btih:c80d50af7d3db9be66a4d0a86db0286e4fd33292&dn=bsd-3-clause.txt BSD 3-Clause
 * Copyright (c) 2011, Terrence Lee <kill889@gmail.com>
 * All rights reserved.
 * 
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are met:
 *     * Redistributions of source code must retain the above copyright
 *       notice, this list of conditions and the following disclaimer.
 *     * Redistributions in binary form must reproduce the above copyright
 *       notice, this list of conditions and the following disclaimer in the
 *       documentation and/or other materials provided with the distribution.
 *     * Neither the name of the <organization> nor the
 *       names of its contributors may be used to endorse or promote products
 *       derived from this software without specific prior written permission.
 * 
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
 * WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
 * DISCLAIMED. IN NO EVENT SHALL <COPYRIGHT HOLDER> BE LIABLE FOR ANY
 * DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
 * (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
 * LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
 * ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
 * SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

var gitGraph = function (canvas, rawGraphList, config) {
	if (!canvas.getContext) {
		return;
	}
	
	if (typeof config === "undefined") {
		config = {
			unitSize: 20,
			lineWidth: 3,
			nodeRadius: 4
		};
	}
	
	var flows = [];
	var graphList = [];
	
	var ctx = canvas.getContext("2d");
	
	var init = function () {
		var maxWidth = 0;
		var i;
		var l = rawGraphList.length;
		var row;
		var midStr;
		
		for (i = 0; i < l; i++) {
			midStr = rawGraphList[i].replace(/\s+/g, " ").replace(/^\s+|\s+$/g, "");
			
			maxWidth = Math.max(midStr.replace(/(\_|\s)/g, "").length, maxWidth);
			
			row = midStr.split("");
			
			graphList.unshift(row);
		}
		
		canvas.width = maxWidth * config.unitSize;
		canvas.height = graphList.length * config.unitSize;
		
		ctx.lineWidth = config.lineWidth;
		ctx.lineJoin = "round";
		ctx.lineCap = "round";
	};
	
	var genRandomStr = function () {
		var chars = "0123456789ABCDEF";
		var stringLength = 6;
		var randomString = '', rnum, i;
		for (i = 0; i < stringLength; i++) {
			rnum = Math.floor(Math.random() * chars.length);
			randomString += chars.substring(rnum, rnum + 1);
		}
		
		return randomString;
	};
	
	var findFlow = function (id) {
		var i = flows.length;
		
		while (i-- && flows[i].id !== id) {}
		
		return i;
	};
	
	var findColomn = function (symbol, row) {
		var i = row.length;
		
		while (i-- && row[i] !== symbol) {}
		
		return i;
	};
	
	var findBranchOut = function (row) {
		if (!row) {
			return -1
		}
		
		var i = row.length;
		
		while (i-- && 
			!(row[i - 1] && row[i] === "/" && row[i - 1] === "|") &&
			!(row[i - 2] && row[i] === "_" && row[i - 2] === "|")) {}
		
		return i;
	}
	
	var genNewFlow = function () {
		var newId;
		
		do {
			newId = genRandomStr();
		} while (findFlow(newId) !== -1);
		
		return {id:newId, color:"#" + newId};
	};
	
	//draw method
	var drawLineRight = function (x, y, color) {
		ctx.strokeStyle = color;
		ctx.beginPath();
		ctx.moveTo(x, y + config.unitSize / 2);
		ctx.lineTo(x + config.unitSize, y + config.unitSize / 2);
		ctx.stroke();
	};
	
	var drawLineUp = function (x, y, color) {
		ctx.strokeStyle = color;
		ctx.beginPath();
		ctx.moveTo(x, y + config.unitSize / 2);
		ctx.lineTo(x, y - config.unitSize / 2);
		ctx.stroke();
	};
	
	var drawNode = function (x, y, color) {
		ctx.strokeStyle = color;
		
		drawLineUp(x, y, color);
		
		ctx.beginPath();
		ctx.arc(x, y, config.nodeRadius, 0, Math.PI * 2, true);
		ctx.fill();
	};
	
	var drawLineIn = function (x, y, color) {
		ctx.strokeStyle = color;
		
		ctx.beginPath();
		ctx.moveTo(x + config.unitSize, y + config.unitSize / 2);
		ctx.lineTo(x, y - config.unitSize / 2);
		ctx.stroke();
	};
	
	var drawLineOut = function (x, y, color) {
		ctx.strokeStyle = color;
		ctx.beginPath();
		ctx.moveTo(x, y + config.unitSize / 2);
		ctx.lineTo(x + config.unitSize, y - config.unitSize / 2);
		ctx.stroke();
	};
	
	var draw = function (graphList) {
		var colomn, colomnIndex, prevColomn, condenseIndex;
		var x, y;
		var color;
		var nodePos, outPos;
		var tempFlow;
		var prevRowLength = 0;
		var flowSwapPos = -1;
		var lastLinePos;
		var i, k, l;
		var condenseCurrentLength, condensePrevLength = 0, condenseNextLength = 0;
		
		var inlineIntersect = false;
		
		//initiate for first row
		for (i = 0, l = graphList[0].length; i < l; i++) {
			if (graphList[0][i] !== "_" && graphList[0][i] !== " ") {
				flows.push(genNewFlow());
			}
		}
		
		y = canvas.height - config.unitSize * 0.5;
		
		//iterate
		for (i = 0, l = graphList.length; i < l; i++) {
			x = config.unitSize * 0.5;
			
			currentRow = graphList[i];
			nextRow = graphList[i + 1];
			prevRow = graphList[i - 1];
			
			flowSwapPos = -1;
			
			condenseCurrentLength = currentRow.filter(function (val) {
				return (val !== " "  && val !== "_")
			}).length;
			
			if (nextRow) {
				condenseNextLength = nextRow.filter(function (val) {
					return (val !== " "  && val !== "_")
				}).length;
			} else {
				condenseNextLength = 0;
			}
			
			//pre process begin
			//use last row for analysing
			if (prevRow) {
				if (!inlineIntersect) {
					//intersect might happen
					for (colomnIndex = 0; colomnIndex < prevRowLength; colomnIndex++) {
						if (prevRow[colomnIndex + 1] && 
							(prevRow[colomnIndex] === "/" && prevRow[colomnIndex + 1] === "|") || 
							((prevRow[colomnIndex] === "_" && prevRow[colomnIndex + 1] === "|") &&
							(prevRow[colomnIndex + 2] === "/"))) {
							
							flowSwapPos = colomnIndex;
							
							//swap two flow
							tempFlow = {id:flows[flowSwapPos].id, color:flows[flowSwapPos].color};
							
							flows[flowSwapPos].id = flows[flowSwapPos + 1].id;
							flows[flowSwapPos].color = flows[flowSwapPos + 1].color;
							
							flows[flowSwapPos + 1].id = tempFlow.id;
							flows[flowSwapPos + 1].color = tempFlow.color;
						}
					}
				}
				
				if (condensePrevLength < condenseCurrentLength &&
					((nodePos = findColomn("*", currentRow)) !== -1 &&
					(findColomn("_", currentRow) === -1))) {
					
					flows.splice(nodePos - 1, 0, genNewFlow());
				}
				
				if (prevRowLength > currentRow.length &&
					(nodePos = findColomn("*", prevRow)) !== -1) {
					
					if (findColomn("_", currentRow) === -1 &&
						findColomn("/", currentRow) === -1 && 
						findColomn("\\", currentRow) === -1) {
						
						flows.splice(nodePos + 1, 1);
					}
				}
			} //done with the previous row
			
			prevRowLength = currentRow.length; //store for next round
			colomnIndex = 0; //reset index
			condenseIndex = 0;
			condensePrevLength = 0;
			while (colomnIndex < currentRow.length) {
				colomn = currentRow[colomnIndex];
				
				if (colomn !== " " && colomn !== "_") {
					++condensePrevLength;
				}
				
				if (colomn === " " && 
					currentRow[colomnIndex + 1] &&
					currentRow[colomnIndex + 1] === "_" &&
					currentRow[colomnIndex - 1] && 
					currentRow[colomnIndex - 1] === "|") {
					
					currentRow.splice(colomnIndex, 1);
					
					currentRow[colomnIndex] = "/";
					colomn = "/";
				}
				
				//create new flow only when no intersetc happened
				if (flowSwapPos === -1 &&
					colomn === "/" &&
					currentRow[colomnIndex - 1] && 
					currentRow[colomnIndex - 1] === "|") {
					
					flows.splice(condenseIndex, 0, genNewFlow());
				}
				
				//change \ and / to | when it's in the last position of the whole row
				if (colomn === "/" || colomn === "\\") {
					if (!(colomn === "/" && findBranchOut(nextRow) === -1)) {
						if ((lastLinePos = Math.max(findColomn("|", currentRow), 
													findColomn("*", currentRow))) !== -1 &&
							(lastLinePos < colomnIndex - 1)) {
							
							while (currentRow[++lastLinePos] === " ") {}
							
							if (lastLinePos === colomnIndex) {
								currentRow[colomnIndex] = "|";
							}
						}
					}
				}
				
				if (colomn === "*" &&
					prevRow && 
					prevRow[condenseIndex + 1] === "\\") {
					flows.splice(condenseIndex + 1, 1);
				}
				
				if (colomn !== " ") {
					++condenseIndex;
				}
				
				++colomnIndex;
			}
			
			condenseCurrentLength = currentRow.filter(function (val) {
				return (val !== " "  && val !== "_")
			}).length;
			
			//do some clean up
			if (flows.length > condenseCurrentLength) {
				flows.splice(condenseCurrentLength, flows.length - condenseCurrentLength);
			}
			
			colomnIndex = 0;
			
			//a little inline analysis and draw process
			while (colomnIndex < currentRow.length) {
				colomn = currentRow[colomnIndex];
				prevColomn = currentRow[colomnIndex - 1];
				
				if (currentRow[colomnIndex] === " ") {
					currentRow.splice(colomnIndex, 1);
					x += config.unitSize;
					
					continue;
				}
				
				//inline interset
				if ((colomn === "_" || colomn === "/") &&
					currentRow[colomnIndex - 1] === "|" &&
					currentRow[colomnIndex - 2] === "_") {
					
					inlineIntersect = true;
					
					tempFlow = flows.splice(colomnIndex - 2, 1)[0];
					flows.splice(colomnIndex - 1, 0, tempFlow);
					currentRow.splice(colomnIndex - 2, 1);
					
					colomnIndex = colomnIndex - 1;
				} else {
					inlineIntersect = false;
				}
				
				color = flows[colomnIndex].color;
				
				switch (colomn) {
					case "_" :
						drawLineRight(x, y, color);
						
						x += config.unitSize;
						break;
						
					case "*" :
						drawNode(x, y, color);
						break;
						
					case "|" :
						drawLineUp(x, y, color);
						break;
						
					case "/" :
						if (prevColomn && 
							(prevColomn === "/" || 
							prevColomn === " ")) {
							x -= config.unitSize;
						}
						
						drawLineOut(x, y, color);
						
						x += config.unitSize;
						break;
						
					case "\\" :
						drawLineIn(x, y, color);
						break;
				}
				
				++colomnIndex;
			}
			
			y -= config.unitSize;
		}
	};
	
	init();
	draw(graphList);
};
// @end-license
