/// Settings sheet: shows the connected server, and sign-out (clears the
/// stored token, returning to the login gate).
library;

import 'package:flutter/material.dart';

import '../app_state.dart';

Future<void> showSettingsSheet(BuildContext context) {
  return showModalBottomSheet<void>(
    context: context,
    showDragHandle: true,
    builder: (sheetContext) => SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(24, 0, 24, 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text('Settings', style: Theme.of(sheetContext).textTheme.titleLarge),
            const SizedBox(height: 16),
            Row(
              children: [
                const Icon(Icons.dns_outlined),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(AppStateScope.of(context).baseUrl,
                      overflow: TextOverflow.ellipsis),
                ),
              ],
            ),
            const SizedBox(height: 8),
            Row(
              children: [
                const Icon(Icons.key_outlined),
                const SizedBox(width: 12),
                Text('API token ${AppStateScope.of(context).hasToken ? 'stored' : 'missing'}'),
              ],
            ),
            const SizedBox(height: 24),
            OutlinedButton.icon(
              style: OutlinedButton.styleFrom(
                foregroundColor: Theme.of(sheetContext).colorScheme.error,
              ),
              onPressed: () async {
                final confirmed = await showDialog<bool>(
                  context: sheetContext,
                  builder: (dialogContext) => AlertDialog(
                    title: const Text('Sign out?'),
                    content: const Text(
                        'The stored API token will be removed from this device.'),
                    actions: [
                      TextButton(
                          onPressed: () => Navigator.pop(dialogContext, false),
                          child: const Text('Cancel')),
                      FilledButton(
                        onPressed: () => Navigator.pop(dialogContext, true),
                        style: FilledButton.styleFrom(
                            backgroundColor:
                                Theme.of(dialogContext).colorScheme.error),
                        child: const Text('Sign out'),
                      ),
                    ],
                  ),
                );
                if (confirmed == true && sheetContext.mounted) {
                  await AppStateScope.of(sheetContext).clearToken();
                  if (sheetContext.mounted) Navigator.pop(sheetContext);
                }
              },
              icon: const Icon(Icons.logout),
              label: const Text('Sign out'),
            ),
          ],
        ),
      ),
    ),
  );
}