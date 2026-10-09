/**
 * HTMX + Alpine.js implementation for RepoRecentCommits
 * 
 * This replaces the Vue component with:
 * - Server-rendered HTML template with data attributes
 * - Alpine.js for client-side reactivity
 * - HTMX for data fetching (polling)
 * - Chart.js for rendering
 */

import {initChart, destroyChart} from '../modules/chart-init.ts';
import {chartJsColors} from '../utils/color.ts';

/**
 * Initialize the recent commits chart
 * 
 * The server template should render:
 * <div id="repo-recent-commits-chart"
 *   data-repo-link="/user/repo"
 *   data-locale-loading-title="..."
 *   data-locale-loading-title-failed="..."
 *   data-locale-loading-info="..."
 * ></div>
 */
export async function initRepoRecentCommits() {
  const container = document.querySelector('#repo-recent-commits-chart');
  if (!container) return;

  const repoLink = container.getAttribute('data-repo-link');
  if (!repoLink) {
    console.error('repo-recent-commits-chart: missing data-repo-link');
    return;
  }

  const locale = {
    loadingTitle: container.getAttribute('data-locale-loading-title') || 'Loading...',
    loadingTitleFailed: container.getAttribute('data-locale-loading-title-failed') || 'Failed to load',
    loadingInfo: container.getAttribute('data-locale-loading-info') || 'Loading...',
  };

  // Build HTML with Alpine.js
  container.innerHTML = `
    <div x-data="{
      isLoading: true,
      errorText: '',
      hasData: false,
      chart: null,
      
      async loadData() {
        this.isLoading = true;
        this.errorText = '';
        this.hasData = false;
        
        try {
          let response;
          do {
            response = await fetch('${repoLink}/activity/recent-commits/data');
            if (response.status === 202) {
              await new Promise(r => setTimeout(r, 1000));
            }
          } while (response.status === 202);
          
          if (!response.ok) {
            throw new Error(response.statusText || 'Request failed');
          }
          
          const dayDataObj = await response.json();
          this.hasData = true;
          this.renderChart(dayDataObj);
          
        } catch (err) {
          this.errorText = err.message || '${locale.loadingTitleFailed}';
        } finally {
          this.isLoading = false;
        }
      },
      
      renderChart(dayDataObj) {
        if (!this.$el) return;
        
        // Process the data into Chart.js format
        const weekValues = Object.values(dayDataObj);
        if (weekValues.length === 0) {
          this.hasData = false;
          return;
        }
        
        const start = weekValues[0].week;
        const end = Date.now();
        
        // Build datasets from the day data
        // The server returns {week: timestamp, commits: number} objects
        const chartData = {
          datasets: [{
            data: weekValues.map((i: any) => ({x: i.week, y: i.commits})),
            label: 'Commits',
            backgroundColor: '${chartJsColors.commits}',
            borderWidth: 0,
            tension: 0.3,
          }],
        };
        
        const options = {
          responsive: true,
          maintainAspectRatio: false,
          scales: {
            x: {
              type: 'time',
              grid: { display: false },
              time: { minUnit: 'week' },
              ticks: { maxRotation: 0, maxTicksLimit: 52 },
            },
            y: {
              ticks: { maxTicksLimit: 6 },
            },
          },
        };
        
        // Cleanup existing chart
        if (this.chart) {
          this.chart.destroy();
          this.chart = null;
        }
        
        // Get canvas and initialize
        const canvas = this.$el.querySelector('#repo-recent-commits-canvas');
        if (canvas) {
          this.chart = initChart(canvas, 'bar', chartData, options);
        }
      }
    }
    x-init="
      // Initial load
      loadData();
      
      // Set up polling every 5 seconds
      const interval = setInterval(() => loadData(), 5000);
      
      // Cleanup on page hide/unload
      const cleanup = () => {
        clearInterval(interval);
        if ($data.chart) {
          $data.chart.destroy();
          $data.chart = null;
        }
      };
      
      window.addEventListener('beforeunload', cleanup);
      document.addEventListener('visibilitychange', () => {
        if (document.hidden) cleanup();
      });
      
      // Also cleanup when this component is removed from DOM
      const observer = new MutationObserver((mutations) => {
        if (!document.body.contains($el)) {
          cleanup();
          observer.disconnect();
        }
      });
      observer.observe(document.body, { childList: true, subtree: true });
    "
    >
      <div class="ui header">
        <template x-if="isLoading">
          <span x-text="'${locale.loadingTitle}'"></span>
        </template>
        <template x-if="!isLoading && errorText">
          <span x-text="'${locale.loadingTitleFailed}'"></span>
        </template>
        <template x-if="!isLoading && !errorText">
          <span>Number of commits in the past year</span>
        </template>
      </div>
      <div class="tw-flex ui segment main-graph">
        <template x-if="isLoading || errorText !== ''">
          <div class="tw-m-auto">
            <template x-if="isLoading">
              <svg class="tw-mr-2 rotate-clockwise" style="width: 16px; height: 16px;" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"/>
              </svg>
              <span x-text="'${locale.loadingInfo}'"></span>
            </template>
            <template x-if="!isLoading">
              <svg style="width: 16px; height: 16px; color: var(--color-red); margin-right: 0.5rem;" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/>
              </svg>
              <span x-text="errorText"></span>
            </template>
          </div>
        </template>
        <template x-if="hasData">
          <canvas id="repo-recent-commits-canvas" x-ref="chartCanvas" role="img"></canvas>
        </template>
      </div>
      <style>
        .main-graph {
          height: 250px;
        }
        .rotate-clockwise {
          animation: chart-rotate 1s linear infinite;
        }
        @keyframes chart-rotate {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }
      </style>
    </div>
  `;
}
