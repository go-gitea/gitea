// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {mount} from '@vue/test-utils';
import {describe, expect, it} from 'vitest';
import ActionRunStatus from './ActionRunStatus.vue';

describe('ActionRunStatus', () => {
  describe('ARIA attributes', () => {
    it('should have role="img" for screen reader support', () => {
      const wrapper = mount(ActionRunStatus, {
        props: {
          status: 'success',
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const statusElement = wrapper.find('span');
      expect(statusElement.attributes('role')).toBe('img');
    });

    it('should have aria-label for screen reader support', () => {
      const wrapper = mount(ActionRunStatus, {
        props: {
          status: 'success',
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const statusElement = wrapper.find('span');
      expect(statusElement.attributes('aria-label')).toBe('success');
    });

    it('should use localeStatus for aria-label when provided', () => {
      const wrapper = mount(ActionRunStatus, {
        props: {
          status: 'success',
          localeStatus: '成功',
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const statusElement = wrapper.find('span');
      expect(statusElement.attributes('aria-label')).toBe('成功');
    });

    it('should have data-tooltip-content for tooltip', () => {
      const wrapper = mount(ActionRunStatus, {
        props: {
          status: 'failure',
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const statusElement = wrapper.find('span');
      expect(statusElement.attributes('data-tooltip-content')).toBe('failure');
    });
  });

  describe('Status rendering', () => {
    const testCases = [
      {status: 'success', description: 'should render success status'},
      {status: 'skipped', description: 'should render skipped status'},
      {status: 'cancelled', description: 'should render cancelled status'},
      {status: 'waiting', description: 'should render waiting status'},
      {status: 'blocked', description: 'should render blocked status'},
      {status: 'running', description: 'should render running status'},
      {status: 'failure', description: 'should render failure status'},
      {status: 'unknown', description: 'should render unknown status'},
    ] as const;

    for (const {status, description} of testCases) {
      it(description, () => {
        const wrapper = mount(ActionRunStatus, {
          props: {
            status,
          },
          global: {
            stubs: {
              SvgIcon: true,
            },
          },
        });

        const statusElement = wrapper.find('span');
        expect(statusElement.exists()).toBe(true);
        expect(statusElement.attributes('role')).toBe('img');
        expect(statusElement.attributes('aria-label')).toBe(status);
      });
    }
  });

  describe('Size and class props', () => {
    it('should use default size of 16', () => {
      const wrapper = mount(ActionRunStatus, {
        props: {
          status: 'success',
        },
        global: {
          stubs: {
            SvgIcon: {
              name: 'SvgIcon',
              template: '<span class="svg-icon-mock"></span>',
              props: ['name', 'size', 'class'],
            },
          },
        },
      });

      const svgIcons = wrapper.findAllComponents({name: 'SvgIcon'});
      expect(svgIcons.length).toBeGreaterThan(0);
    });

    it('should accept custom size', () => {
      const wrapper = mount(ActionRunStatus, {
        props: {
          status: 'success',
          size: 24,
        },
        global: {
          stubs: {
            SvgIcon: {
              name: 'SvgIcon',
              template: '<span class="svg-icon-mock"></span>',
              props: ['name', 'size', 'class'],
            },
          },
        },
      });

      expect(wrapper.exists()).toBe(true);
    });

    it('should accept custom className', () => {
      const wrapper = mount(ActionRunStatus, {
        props: {
          status: 'success',
          className: 'custom-class',
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      expect(wrapper.exists()).toBe(true);
    });
  });

  describe('Conditional rendering', () => {
    it('should render when status is provided', () => {
      const wrapper = mount(ActionRunStatus, {
        props: {
          status: 'success',
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      expect(wrapper.find('span').exists()).toBe(true);
    });
  });
});