const fs = require('fs');

const nodeListFile = 'view/src/components/cluster/NodeList.vue';
let nodeListContent = fs.readFileSync(nodeListFile, 'utf8');
nodeListContent = nodeListContent.replace(/import\s+\{\s*mockNodes\s*\}\s+from\s+['"]\.\.\/\.\.\/composables\/mockData['"]\n?/g, '');
fs.writeFileSync(nodeListFile, nodeListContent, 'utf8');

const useWailsFile = 'view/src/composables/useWails.js';
let useWailsContent = fs.readFileSync(useWailsFile, 'utf8');
useWailsContent = useWailsContent.replace(/import\s+\{[\s\S]*?\}\s*from\s+['"]\.\/mockData['"]\n?/g, '');

// Replace specific fallback assignments
useWailsContent = useWailsContent.replace(/info\.value\s*=\s*mockClusterInfo/g, 'info.value = null');
useWailsContent = useWailsContent.replace(/if\s*\(!metrics\.value\)\s*metrics\.value\s*=\s*mockMetrics/g, 'if (!metrics.value) metrics.value = null');
useWailsContent = useWailsContent.replace(/if\s*\(alerts\.value\.length\s*===\s*0\)\s*alerts\.value\s*=\s*mockAlerts/g, 'if (alerts.value.length === 0) alerts.value = []');
useWailsContent = useWailsContent.replace(/report\.value\s*=\s*mockCostReport/g, 'report.value = null');
useWailsContent = useWailsContent.replace(/topology\.value\s*=\s*mockTopology/g, 'topology.value = null');
useWailsContent = useWailsContent.replace(/logs\.value\s*=\s*mockNodeLogs/g, 'logs.value = []');
useWailsContent = useWailsContent.replace(/nodes\.value\s*=\s*mockNodes/g, 'nodes.value = []');
useWailsContent = useWailsContent.replace(/applications\.value\s*=\s*mockApplications/g, 'applications.value = []');
useWailsContent = useWailsContent.replace(/return mockReport/g, 'return null');

fs.writeFileSync(useWailsFile, useWailsContent, 'utf8');

console.log('Fixed imports and fallback values in useWails.js and NodeList.vue');
