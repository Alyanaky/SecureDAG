<template>
  <div class="nodes-manager">
    <div v-for="node in nodes" :key="node.id" class="node-card">
      <div class="status" :class="node.status"></div>
      <h4>{{ node.id }}</h4>
      <p>CPU: {{ node.cpu }}%</p>
      <p>Memory: {{ node.memory }}%</p>
      <button @click="restartNode(node.id)">⟳ Restart</button>
    </div>
  </div>
</template>

<script>
export default {
  data() {
    return {
      nodes: []
    }
  },
  async created() {
    const res = await this.$api.getNodes()
    this.nodes = res.data
  },
  methods: {
    async restartNode(nodeId) {
      await this.$api.restartNode(nodeId)
      this.nodes = this.nodes.map(n => 
        n.id === nodeId ? {...n, status: 'restarting'} : n
      )
    }
  }
}
</script>

<style scoped>
.node-card {
  border: 1px solid #ccc;
  padding: 1rem;
  margin: 1rem 0;
}
.status {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}
.status.active { background: green; }
.status.restarting { background: orange; }
</style>
