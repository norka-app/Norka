<script setup>
import AppSelect from '../common/AppSelect.vue'
import {
  FIELD_GAP,
  FORM_COLS,
  SPAN_FIVE,
  SPAN_FULL,
  SPAN_HALF,
  SPAN_SEVEN,
  SPAN_THIRD,
  SPAN_TWO_THIRDS,
  plainInputProps,
  requiredInputProps,
} from '../../theme/form-layout'

defineProps({
  form: { type: Object, required: true },
  limits: { type: Object, required: true },
  authOptions: { type: Array, required: true },
  needsKeyFile: { type: Boolean, required: true },
  showsPassword: { type: Boolean, required: true },
  passwordPlaceholder: { type: String, required: true },
  passwordRequired: { type: Boolean, required: true },
  showStoredHint: { type: Boolean, default: false },
  showNotes: { type: Boolean, default: false },
})

defineEmits(['key-file-change'])
</script>

<template>
  <n-grid :cols="FORM_COLS" :x-gap="FIELD_GAP" :y-gap="FIELD_GAP" item-responsive>
    <n-form-item-gi :span="SPAN_HALF" :label="$t('app.modals.jumper.name')" :show-feedback="false">
      <n-input v-model:value="form.name" :maxlength="limits.name" :input-props="requiredInputProps" />
    </n-form-item-gi>
    <n-form-item-gi :span="SPAN_HALF" :label="$t('app.modals.jumper.user')" :show-feedback="false">
      <n-input v-model:value="form.user" :maxlength="limits.user" :input-props="requiredInputProps" />
    </n-form-item-gi>
    <n-form-item-gi :span="SPAN_TWO_THIRDS" :label="$t('app.modals.jumper.host')" :show-feedback="false">
      <n-input v-model:value="form.host" :maxlength="limits.host" :input-props="requiredInputProps" />
    </n-form-item-gi>
    <n-form-item-gi :span="SPAN_THIRD" :label="$t('app.modals.jumper.port')" :show-feedback="false">
      <n-input-number
        v-model:value="form.port"
        :min="1"
        :max="65535"
        :show-button="false"
        style="width: 100%"
      />
    </n-form-item-gi>
    <n-form-item-gi :span="SPAN_HALF" :label="$t('app.modals.jumper.authMethod')" :show-feedback="false">
      <AppSelect v-model="form.authType" :options="authOptions" />
      <n-text v-if="form.authType === 'ssh_agent'" depth="3" class="kit-note">
        {{ $t('app.modals.jumper.sshAgentNote') }}
      </n-text>
    </n-form-item-gi>
    <n-form-item-gi
      v-if="form.authType === 'ssh_agent'"
      :span="SPAN_HALF"
      :label="$t('app.modals.jumper.agentSocketPath')"
      :show-feedback="false"
    >
      <n-input
        v-model:value="form.agentSocketPath"
        :maxlength="limits.agentSocketPath"
        :placeholder="$t('app.modals.jumper.agentSocketPlaceholder')"
        :input-props="plainInputProps"
      />
      <n-text depth="3" class="kit-note">{{ $t('app.modals.jumper.agentSocketNote') }}</n-text>
    </n-form-item-gi>
    <template v-if="needsKeyFile">
      <n-form-item-gi :span="SPAN_SEVEN" :label="$t('app.modals.jumper.sshKeyFile')" :show-feedback="false">
        <n-input-group>
          <n-input
            v-model:value="form.keyPath"
            :maxlength="limits.keyPath"
            :placeholder="$t('app.modals.jumper.keyPathPlaceholder')"
            :input-props="needsKeyFile ? requiredInputProps : plainInputProps"
          />
          <n-button tag="label">
            {{ $t('app.modals.jumper.browse') }}
            <input class="kit-file-input" type="file" @change="$emit('key-file-change', $event)" />
          </n-button>
        </n-input-group>
        <n-text depth="3" class="kit-note">{{ $t('app.modals.jumper.keyFileNote') }}</n-text>
      </n-form-item-gi>
      <n-form-item-gi :span="SPAN_FIVE" :label="$t('app.modals.jumper.password')" :show-feedback="false">
        <n-input
          v-model:value="form.password"
          type="password"
          :maxlength="limits.password"
          :placeholder="passwordPlaceholder"
          :input-props="passwordRequired ? requiredInputProps : plainInputProps"
        />
        <n-text v-if="showStoredHint" depth="3" class="kit-note">{{ $t('app.modals.jumper.passwordStoredHint') }}</n-text>
      </n-form-item-gi>
    </template>
    <n-form-item-gi
      v-else-if="showsPassword"
      :span="SPAN_FULL"
      :label="$t('app.modals.jumper.password')"
      :show-feedback="false"
    >
      <n-input
        v-model:value="form.password"
        type="password"
        :maxlength="limits.password"
        :placeholder="passwordPlaceholder"
        :input-props="passwordRequired ? requiredInputProps : plainInputProps"
      />
      <n-text v-if="showStoredHint" depth="3" class="kit-note">{{ $t('app.modals.jumper.passwordStoredHint') }}</n-text>
    </n-form-item-gi>
    <n-form-item-gi v-if="showNotes" :span="SPAN_FULL" :label="$t('app.modals.jumper.notes')" :show-feedback="false">
      <n-input
        v-model:value="form.notes"
        type="textarea"
        :rows="2"
        :maxlength="limits.notes"
        :input-props="plainInputProps"
      />
    </n-form-item-gi>
  </n-grid>
</template>
