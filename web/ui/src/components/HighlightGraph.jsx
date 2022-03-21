import { useState, useCallback, useRef, useEffect } from 'react';
import ForceGraph3D from 'react-force-graph-3d';
import {
  CSS2DRenderer,
  CSS2DObject
} from "three/examples/jsm/renderers/CSS2DRenderer.js";

const HighlightGraph = ({ data }) => {
  const NODE_R = 8;
  const extraRenderers = [new CSS2DRenderer()];

  const [highlightNodes, setHighlightNodes] = useState(new Set());
  const [highlightLinks, setHighlightLinks] = useState(new Set());
  const [_, setHoverNode] = useState(null);
  const fgRef = useRef();

  const { links } = data;

  const updateHighlight = () => {
    setHighlightNodes(highlightNodes);
    setHighlightLinks(highlightLinks);
  };

  const handleNodeHover = (node) => {
    highlightNodes.clear();
    highlightLinks.clear();
    if (node) {
      console.log(node);
      highlightNodes.add(node);
      node.neighbors.forEach((neighbor) => highlightNodes.add(neighbor));
      node.links.forEach((link) => highlightLinks.add(link));
    }

    setHoverNode(node || null);
    updateHighlight();
  };

  const handleLinkHover = (link) => {
    highlightNodes.clear();
    highlightLinks.clear();

    if (link) {
      highlightLinks.add(link);
      highlightNodes.add(link.source);
      highlightNodes.add(link.target);
    }

    updateHighlight();
  };

  useEffect(() => {
    let minValue = Infinity;
    let maxValue = -Infinity;
    for (const l of links) {
      minValue = Math.min(minValue, l.value);
      maxValue = Math.max(maxValue, l.value);
    }

    console.log(minValue, maxValue);

    if (fgRef && fgRef.current) {
      fgRef.current.d3Force('link').strength(0.05).distance((l) => {
        const ratio = (l.value - minValue) / (maxValue - minValue);
        const newDistance = maxValue - l.value;

        console.log(l, ratio, newDistance);

        return newDistance;
      });
    }
  }, [links]);

  return (
    <div>
      <ForceGraph3D
        ref={fgRef}
        nodeRelSize={6}
        nodeVal={(n) => n.score * n.score}
        linkWidth={1}
        linkDirectionalParticles={4}
        linkDirectionalParticleWidth={(link) => {
          let matching = Array.from(highlightLinks).filter((l) => {
            return l.source === link.source.id && l.target === link.target.id
          })
          return matching.length > 0 ? 4 : 0;
        }
        }
        extraRenderers={extraRenderers}
        graphData={data}
        nodeAutoColorBy="group"
        nodeThreeObject={(node) => {
          const nodeEl = document.createElement('div');
          nodeEl.textContent = `${node.id}`;
          nodeEl.style.color = node.color;
          nodeEl.className = 'node-label';
          return new CSS2DObject(nodeEl);
        }}
        nodeThreeObjectExtend={true}
        onNodeHover={handleNodeHover}
        onLinkHover={handleLinkHover}
        onNodeDragEnd={node => {
          node.fx = node.x;
          node.fy = node.y;
          node.fz = node.z;
        }}
      />
    </div>
  )
};

export default HighlightGraph;
