import React, {useCallback, useMemo, useRef} from 'react';
import ReactFlow, {
    Node,
    Edge,
    Controls,
    Background,
    useNodesState,
    useEdgesState,
    NodeTypes,
    Position,
    MarkerType,
    addEdge,
    ReactFlowInstance,
    ReactFlowProvider,
    Connection,
} from 'react-flow-renderer';
import {Component} from '../types/api';
import ComponentNode from './ComponentNode';
import ELK from 'elkjs/lib/elk.bundled.js';

interface DependencyGraphProps {
    components: Component[];
    onNodeClick: (component: Component) => void;
}

const nodeTypes: NodeTypes = {
    component: ComponentNode,
};

const elk = new ELK();

const elkOptions: Record<string, string> = {
    'elk.algorithm': 'layered',
    'elk.layered.spacing.nodeNodeBetweenLayers': '70',
    'elk.spacing.nodeNode': '100',
    'elk.spacing.edgeNode': '80',
    'elk.spacing.edgeEdge': '50',
    'elk.layered.spacing.edgeNodeBetweenLayers': '0',
    'elk.layered.spacing.edgeEdgeBetweenLayers': '60',
    'elk.layered.crossingMinimization.semiInteractive': 'true',
    'elk.layered.nodePlacement.strategy': 'NETWORK_SIMPLEX', // stable ordering
    'elk.considerModelOrder': 'true',                        // follow given input order
};

const FIT_VIEW_OPTIONS = {padding: 0.2} as const;

const getLayoutedElements = (
    nodes: Node[],
    edges: Edge[],
    options: Record<string, string>
) => {
    const isHorizontal = options?.['elk.direction'] === 'RIGHT';

    const graph = {
        id: 'root',
        layoutOptions: options,
        children: nodes.map((node) => ({
            id: node.id,
            width: 150,
            height: 64,
        })),
        edges: edges.map((e) => ({id: e.id, sources: [e.source], targets: [e.target]})),
    } as any;

    return elk
        .layout(graph)
        .then((layoutedGraph: any) => ({
            nodes: nodes.map((n) => {
                const laidOut = layoutedGraph.children.find((c: any) => c.id === n.id) || {};
                return {
                    ...n,
                    position: {x: laidOut.x || 0, y: laidOut.y || 0},
                    targetPosition: isHorizontal ? Position.Left : Position.Top,
                    sourcePosition: isHorizontal ? Position.Right : Position.Bottom,
                } as Node;
            }),
            edges,
        }))
        .catch((err: any) => {
            console.error(err);
            return {nodes, edges};
        });
};

const computeGraphSignature = (components: Component[]): string => {
    const nodeIds = components.map((c) => c.id.toString()).sort().join('|');
    const edgePairs: string[] = [];
    components.forEach((c) => {
        c.depends_on?.forEach((depId) => edgePairs.push(`${c.id}->${depId}`));
    });
    edgePairs.sort();
    return `${nodeIds}||${edgePairs.join('|')}`;
};

const DependencyGraphInner: React.FC<DependencyGraphProps> = ({components, onNodeClick}) => {
    // Build initial nodes/edges from the provided components
    const initialNodes: Node[] = useMemo(() => (
        components.map((comp) => ({
            id: comp.id.toString(),
            type: 'component',
            position: {x: 0, y: 0},
            data: {component: comp, onClick: onNodeClick},
            sourcePosition: Position.Bottom,
            targetPosition: Position.Top,
        }))
    ), [components, onNodeClick]);

    const initialEdges: Edge[] = useMemo(() => {
        const edgeSet = new Set<string>();
        const es: Edge[] = [];
        components.forEach((comp) => {
            comp.depends_on?.forEach((depId) => {
                const edgeId = `${comp.id}-${depId}`;
                if (!edgeSet.has(edgeId)) {
                    edgeSet.add(edgeId);
                    es.push({
                        id: edgeId,
                        source: comp.id.toString(),
                        target: depId.toString(),
                        type: 'smoothstep',
                        style: {stroke: '#000', strokeWidth: 2},
                        markerEnd: {
                            type: MarkerType.ArrowClosed,
                            color: '#000',
                        },
                    });
                }
            });
        });
        return es;
    }, [components]);

    const [nodes, setNodes, onNodesChange] = useNodesState([]);
    const [edges, setEdges, onEdgesChange] = useEdgesState([]);

    const rfInstanceRef = useRef<ReactFlowInstance | null>(null);
    const onInit = useCallback((instance: ReactFlowInstance) => {
        rfInstanceRef.current = instance;
    }, []);

    const onConnect = useCallback((params: Connection) => setEdges((eds) => addEdge(params, eds)), [setEdges]);

    const prevStructureKeyRef = useRef<string | null>(null);

    // Re-layout when the graph structure (ids/edges) changes; otherwise just update data
    React.useEffect(() => {
        const structureKey = computeGraphSignature(components);
        const structureChanged = structureKey !== prevStructureKeyRef.current;

        if (structureChanged) {
            const opts = {'elk.direction': 'DOWN', ...elkOptions};
            getLayoutedElements(initialNodes, initialEdges, opts).then(({
                                                                            nodes: layoutedNodes,
                                                                            edges: layoutedEdges
                                                                        }) => {
                setNodes(layoutedNodes);
                setEdges(layoutedEdges);
                rfInstanceRef.current?.fitView(FIT_VIEW_OPTIONS);
                prevStructureKeyRef.current = structureKey;
            });
        } else {
            setNodes((prev) => prev.map((node) => {
                const component = components.find((c) => c.id.toString() === node.id);
                if (!component) return node;
                return {...node, data: {...node.data, component, onClick: onNodeClick}} as Node;
            }));
        }
    }, [components, onNodeClick, initialNodes, initialEdges, setNodes, setEdges]);

    return (
        <div style={{width: '100%', height: '100%'}}>
            <ReactFlow
                nodes={nodes}
                edges={edges}
                onNodesChange={onNodesChange}
                onEdgesChange={onEdgesChange}
                onConnect={onConnect}
                onInit={onInit}
                nodeTypes={nodeTypes}
                fitView
                fitViewOptions={FIT_VIEW_OPTIONS}
                minZoom={0.1}
                maxZoom={2}
                nodesDraggable={false}
                nodesConnectable={false}
                elementsSelectable={false}
                selectNodesOnDrag={false}
                zoomOnDoubleClick={false}
                onNodeClick={(event, node) => {
                    const comp = components.find(c => c.id.toString() === node.id);
                    if (comp) onNodeClick(comp);
                }}
            >
                <Controls/>
                <Background/>
            </ReactFlow>
        </div>
    );
};

const DependencyGraph: React.FC<DependencyGraphProps> = (props) => (
    <ReactFlowProvider>
        <DependencyGraphInner {...props} />
    </ReactFlowProvider>
);

export default DependencyGraph;
