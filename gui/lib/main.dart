import 'package:flutter/material.dart';
import 'pages/dashboard_page.dart';
import 'pages/tasks_page.dart';
import 'pages/skills_page.dart';
import 'pages/settings_page.dart';
import 'services/api_client.dart';

void main() {
  runApp(const NovelAgentApp());
}

class NovelAgentApp extends StatelessWidget {
  const NovelAgentApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'AI Novel Agent',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        brightness: Brightness.dark,
        primaryColor: const Color(0xFFCBA6F7),
        scaffoldBackgroundColor: const Color(0xFF1E1E2E),
        colorScheme: const ColorScheme.dark(
          primary: Color(0xFFCBA6F7),
          secondary: Color(0xFF89B4FA),
          surface: Color(0xFF181825),
        ),
      ),
      home: const MainShell(),
    );
  }
}

class MainShell extends StatefulWidget {
  const MainShell({super.key});

  @override
  State<MainShell> createState() => _MainShellState();
}

class _MainShellState extends State<MainShell> {
  int _selectedIndex = 0;

  final _pages = const [
    DashboardPage(),
    TasksPage(),
    SkillsPage(),
    SettingsPage(),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Row(
        children: [
          NavigationRail(
            selectedIndex: _selectedIndex,
            onDestinationSelected: (i) => setState(() => _selectedIndex = i),
            labelType: NavigationRailLabelType.all,
            backgroundColor: const Color(0xFF181825),
            selectedIconTheme: const IconThemeData(color: Color(0xFFCBA6F7)),
            indicatorColor: const Color(0xFF313244),
            destinations: const [
              NavigationRailDestination(
                icon: Icon(Icons.dashboard_outlined),
                selectedIcon: Icon(Icons.dashboard),
                label: Text('Dashboard'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.auto_stories_outlined),
                selectedIcon: Icon(Icons.auto_stories),
                label: Text('Tasks'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.psychology_outlined),
                selectedIcon: Icon(Icons.psychology),
                label: Text('Skills'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.settings_outlined),
                selectedIcon: Icon(Icons.settings),
                label: Text('Settings'),
              ),
            ],
          ),
          const VerticalDivider(width: 1, color: Color(0xFF313244)),
          Expanded(child: _pages[_selectedIndex]),
        ],
      ),
    );
  }
}
