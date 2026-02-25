import {ConfigFormValueMapper} from './config.ts';

test('ConfigFormValueMapper', () => {
  document.body.innerHTML = `
<form>
    <input id="checkbox-unrelated" type="checkbox" value="v-unrelated" checked>

    <!-- top-level key -->
    <input name="k1" type="checkbox" value="v-key-only" data-config-dyn-key="k1" data-config-value-json="true" data-config-value-type="boolean">
    <input type="hidden" data-config-dyn-key="k2" data-config-value-json='"k2-val"'>
    <input name="k2">
    <textarea name="repository.open-with.editor-apps"> a = b\n</textarea>

    <!-- sub key -->
    <input type="hidden" data-config-dyn-key="struct" data-config-value-json='{"SubBoolean": true, "SubTimestamp": 123456789, "OtherKey": "other-value"}'>
    <input name="struct.SubBoolean" type="checkbox" data-config-value-type="boolean">
    <input name="struct.SubTimestamp" type="datetime-local" data-config-value-type="timestamp">
    <textarea name="struct.NewKey">new-value</textarea>
</form>
`;

  const form = document.querySelector('form')!;
  const mapper = new ConfigFormValueMapper(form);
  mapper.fillFromSystemConfig();
  const formData = mapper.collectToFormData();
  const result: Record<string, string> = {};
  const keys = [], values = [];
  for (const [key, value] of formData.entries()) {
    if (key === 'key') keys.push(value as string);
    if (key === 'value') values.push(value as string);
  }
  for (let i = 0; i < keys.length; i++) {
    result[keys[i]] = values[i];
  }
  expect(result).toEqual({
    'k1': 'true',
    'k2': '"k2-val"',
    'repository.open-with.editor-apps': '[{"DisplayName":"a","OpenURL":"b"}]', // TODO: OPEN-WITH-EDITOR-APP-JSON: it must match backend
    'struct': '{"SubBoolean":true,"SubTimestamp":123456780,"OtherKey":"other-value","NewKey":"new-value"}',
  });
});
