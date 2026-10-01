import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:tracking/main.dart';
import 'package:tracking/src/app_state.dart';

Future<AppState> _freshState(Map<String, Object> values) async {
  SharedPreferences.setMockInitialValues(values);
  return AppState(await SharedPreferences.getInstance());
}

void main() {
  testWidgets('no stored token shows the login gate',
      (WidgetTester tester) async {
    final state = await _freshState({});
    await tester.pumpWidget(TrackApp(state: state));

    expect(find.text('Connect'), findsOneWidget);
    expect(find.text('Server URL'), findsOneWidget);
    expect(find.text('API token'), findsOneWidget);
  });

  testWidgets('empty token is rejected with a validation message',
      (WidgetTester tester) async {
    final state = await _freshState({});
    await tester.pumpWidget(TrackApp(state: state));

    await tester.tap(find.text('Connect'));
    await tester.pump();

    // The URL field is pre-seeded with the default server, so only the
    // empty-token validator fires; the gate stays up.
    expect(find.text('Enter the token'), findsOneWidget);
  });

  testWidgets('stored token goes straight to the main UI',
      (WidgetTester tester) async {
    final state = await _freshState({
      'api_token': 'test-token',
      'base_url': 'https://example.invalid',
    });
    await tester.pumpWidget(TrackApp(state: state));

    expect(find.text('Measure'), findsOneWidget);
    expect(find.text('Meals'), findsOneWidget);
    expect(find.text('Fasts'), findsOneWidget);
    expect(find.text('Workouts'), findsOneWidget);
  });
}