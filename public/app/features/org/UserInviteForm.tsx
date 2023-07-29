import React from 'react';

import { locationUtil, SelectableValue } from '@grafana/data';
import { Stack } from '@grafana/experimental';
import { locationService } from '@grafana/runtime';
import { Button, LinkButton, Input, Switch, RadioButtonGroup, Form, Field, InputControl, FieldSet } from '@grafana/ui';
import { getConfig } from 'app/core/config';
import { OrgRole, useDispatch } from 'app/types';

import { addInvitee } from '../invites/state/actions';

const roles: Array<SelectableValue<OrgRole>> = Object.values(OrgRole).map((r) => ({
  label: r === OrgRole.None ? 'No basic role' : r,
  value: r,
  tooltip:
    r === OrgRole.None
      ? 'This role has no permissions by default. You may still add specific permissions with the help of RBAC.'
      : undefined,
}));

export interface FormModel {
  role: OrgRole;
  name: string;
  loginOrEmail?: string;
  sendEmail: boolean;
  email: string;
}

const defaultValues: FormModel = {
  name: '',
  email: '',
  role: OrgRole.Editor,
  sendEmail: true,
};

export const UserInviteForm = () => {
  const dispatch = useDispatch();

  const onSubmit = async (formData: FormModel) => {
    await dispatch(addInvitee(formData)).unwrap();
    locationService.push('/admin/users/');
  };

  return (
    <Form defaultValues={defaultValues} onSubmit={onSubmit}>
      {({ register, control, errors }) => {
        return (
          <>
            <FieldSet>
              <Field
                invalid={!!errors.loginOrEmail}
                error={!!errors.loginOrEmail ? 'Email or username is required' : undefined}
                label="Email or username"
              >
                <Input {...register('loginOrEmail', { required: true })} placeholder="email@example.com" />
              </Field>
              <Field invalid={!!errors.name} label="Name">
                <Input {...register('name')} placeholder="(optional)" />
              </Field>
              <Field invalid={!!errors.role} label="Role">
                <InputControl
                  render={({ field: { ref, ...field } }) => <RadioButtonGroup {...field} options={roles} />}
                  control={control}
                  name="role"
                />
              </Field>
              <Field label="Send invite email">
                <Switch id="send-email-switch" {...register('sendEmail')} />
              </Field>
            </FieldSet>
            <Stack>
              <Button type="submit">Submit</Button>
              <LinkButton href={locationUtil.assureBaseUrl(getConfig().appSubUrl + '/admin/users')} variant="secondary">
                Back
              </LinkButton>
            </Stack>
          </>
        );
      }}
    </Form>
  );
};

export default UserInviteForm;
