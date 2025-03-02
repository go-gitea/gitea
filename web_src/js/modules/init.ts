export class InitPerformanceTracer {
  results: {name: string, dur: number}[] = [];
  recordCall(name: string, func: ()=>void) {
    const start = performance.now();
    func();
    this.results.push({name, dur: performance.now() - start});
  }
  printResults() {
    this.results = this.results.sort((a, b) => b.dur - a.dur);
    for (let i = 0; i < 20 && i < this.results.length; i++) {
      console.info(`performance trace: ${this.results[i].name} ${this.results[i].dur.toFixed(3)}`);
    }
  }
}

export function callInitFunctions(functions: (() => any)[]): InitPerformanceTracer | null {
  // Start performance trace by accessing a URL by "https://localhost/?_ui_performance_trace=1" or "https://localhost/?key=value&_ui_performance_trace=1"
  // It is a quick check, no side effect so no need to do slow URL parsing.
  const perfTracer = !window.location.search.includes('_ui_performance_trace=1') ? null : new InitPerformanceTracer();
  if (perfTracer) {
    for (const func of functions) perfTracer.recordCall(func.name, func);
  } else {
    for (const func of functions) func();
  }
  return perfTracer;
}
