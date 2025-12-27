# Implement Auto Refresh on Startup with Customizable Sources

I will implement a feature that allows users to configure the app to automatically refresh specific RSS sources when the application starts.

## 1. Data & Storage Layer
### Modify `src/types/index.ts`
- Define a new interface `RSSStartupSettings`:
  ```typescript
  export interface RSSStartupSettings {
    enabled: boolean;
    sourceIds: number[]; // List of source IDs to refresh
  }
  ```

### Modify `src/services/SettingsService.ts`
- Add a new storage key `RSS_STARTUP_SETTINGS`.
- Implement `getRSSStartupSettings()`: Returns the settings or default `{ enabled: false, sourceIds: [] }`.
- Implement `saveRSSStartupSettings(settings)`: Saves to `AsyncStorage`.

## 2. State Management
### Modify `src/contexts/RSSSourceContext.tsx`
- Add state for `startupSettings`.
- Add `loadStartupSettings()` to initialization.
- Add `updateStartupSettings(settings)` to update state and call service.
- Implement `triggerStartupRefresh()`:
  - Check if `startupSettings.enabled` is true.
  - If true, filter `rssSources` based on `startupSettings.sourceIds`.
  - Call `syncSources(ids)` for the matching sources.

## 3. Application Lifecycle
### Modify `src/navigation/AppNavigator.tsx`
- In `RootNavigator`, use `useRSSSource`.
- Inside the `useEffect` that handles `SplashScreen.hideAsync`:
  - Call `triggerStartupRefresh()` immediately before or after hiding the splash screen.
  - Ensure this only runs once per app launch.

## 4. UI Implementation
### Create `src/screens/Settings/RSSStartupSettingsScreen.tsx`
- **Main Toggle**: "开机自动刷新" (Auto Refresh on Startup).
- **Source Selection List**:
  - Visible only when toggle is ON.
  - List all available RSS sources with checkboxes.
  - "Select All" / "Deselect All" helper buttons.
- **Save Logic**: Updates context and persists data.

### Modify `src/navigation/types.ts`
- Add `RSSStartupSettings` to `UserStackParamList`.

### Modify `src/navigation/AppNavigator.tsx`
- Register `RSSStartupSettingsScreen` in `UserStackNavigator`.

### Modify `src/screens/Mine/MineScreen.tsx`
- Add a new `SettingItem` under "工具与服务" (Tools & Services) or "阅读与内容" named "RSS 启动刷新" (RSS Startup Refresh) linking to the new screen.
